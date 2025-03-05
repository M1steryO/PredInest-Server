package registration

import (
	"PredInvest/internal/lib/logger/sl"
	"PredInvest/internal/storage"
	"errors"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"net/http"

	resp "PredInvest/internal/lib/api/response"
)

type Request struct {
	Email    string `json:"email" validate:"required,email"`
	Name     string `json:"name,omitempty"`
	Password string `json:"password" validate:"required,min=8"`
}

type Response struct {
	resp.Response
}

//go:generate go run github.com/vektra/mockery/v2@v2.28.2 --name=UserSaver
type UserSaver interface {
	SaveUser(email string, name string, password string) (int64, error)
}

func New(log *slog.Logger, userSaver UserSaver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const op = "handlers.user.registration.New"

		log := log.With(
			slog.String("op", op),
			slog.String("request_id", middleware.GetReqID(r.Context())),
		)

		var req Request
		err := render.DecodeJSON(r.Body, &req)
		if errors.Is(err, io.EOF) {
			// Такую ошибку встретим, если получили запрос с пустым телом.
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

		password, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)

			log.Error("failed to generate password", sl.Err(err))
			render.JSON(w, r, resp.Error("failed to add user"))
			return
		}
		id, err := userSaver.SaveUser(req.Email, req.Name, string(password))
		if errors.Is(err, storage.ErrUserExists) {
			log.Info("user already exists", slog.String("user", req.Email))
			w.WriteHeader(http.StatusConflict)
			render.JSON(w, r, resp.Error("user already exists"))
			return
		}
		if err != nil {
			log.Error("failed to add user", err.Error())

			render.JSON(w, r, resp.Error("failed to add user"))

			return
		}

		log.Info("user added", slog.Int64("id", id))
		w.WriteHeader(http.StatusCreated)
		responseOK(w, r)
	}
}

func responseOK(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, Response{
		Response: resp.OK(),
	})
}
