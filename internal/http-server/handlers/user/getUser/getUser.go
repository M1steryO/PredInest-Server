package getUser

import (
	resp "PredInvest/internal/lib/api/response"
	"PredInvest/internal/lib/logger/sl"
	"PredInvest/internal/storage"
	"PredInvest/internal/storage/postgres"
	"errors"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"log/slog"
	"net/http"
	"strconv"
)

const SecretKey = "secret"

type Response struct {
	resp.Response
	postgres.User `json:"user,omitempty"`
}
type UserGetter interface {
	GetUserById(id int64) (postgres.User, error)
}

func New(log *slog.Logger, userGetter UserGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.user.logout.New"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		cookie, err := r.Cookie("jwt")
		if err != nil {
			log.Error("could not get cookie", sl.Err(err))
			w.WriteHeader(http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("internal error"))
			return
		}

		token, err := jwt.ParseWithClaims(cookie.Value, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(SecretKey), nil
		})
		if err != nil {
			log.Error("could not parse jwt", sl.Err(err))
			w.WriteHeader(http.StatusUnauthorized)
			render.JSON(w, r, resp.Error("user not authorized"))
		}

		claims := token.Claims.(*jwt.StandardClaims)

		var user postgres.User
		userId, err := strconv.ParseInt(claims.Issuer, 10, 64)
		if err != nil {
			log.Error("could not parse user id", sl.Err(err))
			w.WriteHeader(http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("internal error"))
			return
		}
		user, err = userGetter.GetUserById(userId)

		if err != nil {
			if errors.Is(storage.ErrUserNotFound, err) {
				w.WriteHeader(http.StatusNotFound)
				log.Error("user not found", sl.Err(err))
				render.JSON(w, r, resp.Error("user not found"))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			log.Error("failed to get user", sl.Err(err))
			render.JSON(w, r, resp.Error("internal error"))
			return
		}

		responseOK(w, r, user)
	}
}

func responseOK(w http.ResponseWriter, r *http.Request, user postgres.User) {
	render.JSON(w, r, Response{
		Response: resp.OK(),
		User:     user,
	})
}
