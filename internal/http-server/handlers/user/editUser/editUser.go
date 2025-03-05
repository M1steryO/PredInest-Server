package editUser

import (
	resp "PredInvest/internal/lib/api/response"
	"PredInvest/internal/lib/logger/sl"
	"PredInvest/internal/storage"
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

const SecretKey = "secret"

type Request struct {
	Email    string `json:"email" validate:"email"`
	Name     string `json:"name"`
	Password string `json:"password" validate:"min=8"`
}

type Response struct {
	resp.Response
}
type UserEditor interface {
	UpdateUserById(id int64, name string, email string, password string) (int64, error)
}

func New(log *slog.Logger, userEditor UserEditor) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.user.editUser.New"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)
		var req Request
		err := render.DecodeJSON(r.Body, &req)
		if errors.Is(err, io.EOF) {
			// Такую ошибку встретим, если получили запрос с пустым телом.
			log.Error("request body is empty", sl.Err(err))
			render.JSON(w, r, resp.Error("empty request"))
			return
		}
		if err != nil {
			log.Error("failed to decode request body", sl.Err(err))
			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, resp.Error("failed to decode request"))
			return
		}

		password, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error("failed to generate password", sl.Err(err))
			render.JSON(w, r, resp.Error("failed to add user"))
			return
		}

		cookie, err := r.Cookie("jwt")
		if err != nil {
			log.Error("could not get cookie", sl.Err(err))
			w.WriteHeader(http.StatusUnauthorized)
			render.JSON(w, r, resp.Error("user not authorized"))
			return
		}

		token, err := jwt.ParseWithClaims(cookie.Value, &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(SecretKey), nil
		})
		if err != nil {
			log.Error("could not parse jwt", slog.String("error", err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("internal error"))
		}

		claims := token.Claims.(*jwt.StandardClaims)

		userId, err := strconv.ParseInt(claims.Issuer, 10, 64)
		if err != nil {
			log.Error("could not parse user id", sl.Err(err))
			w.WriteHeader(http.StatusInternalServerError)
			render.JSON(w, r, resp.Error("internal error"))
			return
		}
		id, err := userEditor.UpdateUserById(userId, req.Name, req.Email, string(password))

		if err != nil {
			fmt.Println(err)
			if errors.Is(err, storage.ErrUserExists) {
				w.WriteHeader(http.StatusBadRequest)
				log.Error("email already exists", sl.Err(err))
				render.JSON(w, r, resp.Error("user already exists"))
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			log.Error("failed to update user", sl.Err(err))
			render.JSON(w, r, resp.Error("internal error"))
			return
		}
		log.Info("user updated", slog.Int64("id", id))

		responseOK(w, r)
	}
}

func responseOK(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, Response{
		Response: resp.OK(),
	})
}
