package login

import (
	"PredInvest/internal/lib/logger/sl"
	"PredInvest/internal/storage"
	"errors"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	resp "PredInvest/internal/lib/api/response"
	"PredInvest/internal/storage/postgres"
)

const SecretKey = "secret"

type Request struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type Response struct {
	resp.Response
	postgres.User
}

//go:generate go run github.com/vektra/mockery/v2@v2.28.2 --name=UserSaver
type UserGetter interface {
	GetUser(email string) (postgres.User, error)
}

func New(log *slog.Logger, userGetter UserGetter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.user.login.New"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req Request
		err := render.DecodeJSON(r.Body, &req)
		if errors.Is(err, io.EOF) {
			w.WriteHeader(http.StatusBadRequest)
			log.Error("request body is empty")
			render.JSON(w, r, resp.Error("empty request"))

			return
		}
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Error("failed to decode request body", sl.Err(err))
			render.JSON(w, r, resp.Error("failed to decode request"))
			return
		}

		log.Info("request body decoded", slog.Any("request", req))

		if err := validator.New().Struct(req); err != nil {
			var validateErr validator.ValidationErrors
			errors.As(err, &validateErr)

			log.Error("invalid request", sl.Err(err))

			w.WriteHeader(http.StatusBadRequest)
			render.JSON(w, r, resp.ValidationError(validateErr))

			return
		}

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error("failed to generate password", sl.Err(err))
			render.JSON(w, r, resp.Error("internal error"))
			return
		}
		user, err := userGetter.GetUser(req.Email)

		if err != nil {
			if errors.Is(storage.ErrUserNotFound, err) {
				w.WriteHeader(http.StatusNotFound)
				log.Error("user not found", sl.Err(err))
				render.JSON(w, r, resp.Error("internal error"))
				return
			}
			log.Error("failed to get user", sl.Err(err))
			render.JSON(w, r, resp.Error("internal error"))
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Error("invalid password", slog.Int64("id", user.ID), sl.Err(err))
			render.JSON(w, r, resp.Error("invalid password"))
			return
		}

		log.Info("get user success", slog.Int64("id", user.ID))

		claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.StandardClaims{
			Issuer:    strconv.Itoa(int(user.ID)),
			ExpiresAt: time.Now().Add(time.Hour * 24).Unix(), //1 day
		})

		token, err := claims.SignedString([]byte(SecretKey))

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error("could not login", slog.Int64("id", user.ID), sl.Err(err))
			render.JSON(w, r, resp.Error("internal error"))
			return
		}

		cookie := http.Cookie{
			Name:     "jwt",
			Value:    token,
			Expires:  time.Now().Add(time.Hour * 24),
			HttpOnly: true,
		}
		http.SetCookie(w, &cookie)
		log.Info("cookie is set", slog.Int64("id", user.ID))
		responseOK(w, r, user)
	}
}

func responseOK(w http.ResponseWriter, r *http.Request, user postgres.User) {
	render.JSON(w, r, Response{
		Response: resp.OK(),
		User:     user,
	})
}
