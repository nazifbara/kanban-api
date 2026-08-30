package main

import (
	"errors"
	"net/http"
	"time"

	"github.com/nazifbara/kanban-api/internal/apierrors"
	"github.com/nazifbara/kanban-api/internal/auth"
	"github.com/nazifbara/kanban-api/internal/database"
)

func (s *server) handlerLogin(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[auth.Params](r)
	if err != nil {
		s.respondWithError(r.Context(), w, malformedBodyErr)
	}

	err = auth.ValidateParams(params)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}

	dbIdentity, err := s.store.GetIdentityByEmail(r.Context(), params.Email)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}

	v, err := auth.CheckPasswordHash(params.Password, dbIdentity.PasswordHash)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusNotFound, err))
		return
	}

	if !v {
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusNotFound, errors.New("incorrect password")))
		return
	}

	token, err := auth.MakeJWT(dbIdentity.ID, s.jwtSecret, time.Hour)
	if err != nil {
		s.respondWithError(r.Context(), w, err)
		return
	}

	refreshToken, err := s.store.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     auth.MakeRefreshToken(),
		UserID:    dbIdentity.ID,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	})
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromDBErr(err))
		return
	}

	s.respondWithJSON(w, http.StatusOK, auth.IdentityWithToken{
		ID:           dbIdentity.ID,
		Token:        token,
		RefreshToken: refreshToken.Token,
	})
}
