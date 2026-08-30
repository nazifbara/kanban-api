package main

import (
	"net/http"

	"github.com/nazifbara/kanban-api/internal/apierrors"
	"github.com/nazifbara/kanban-api/internal/auth"
	"github.com/nazifbara/kanban-api/internal/database"
	"github.com/nazifbara/kanban-api/internal/users"
)

func (s *server) handlerSignUp(w http.ResponseWriter, r *http.Request) {
	params, err := decodeJSONBody[users.UserParam](r)
	if err != nil {
		s.respondWithError(r.Context(), w, malformedBodyErr)
		return
	}

	err = users.ValidateParams(params)
	if err != nil {
		s.respondWithError(r.Context(), w, apierrors.FromErr(http.StatusBadRequest, err))
		return
	}

	var user database.User
	err = s.store.execTx(r.Context(), func(q *database.Queries) error {
		passwordHash, err := auth.HashPassword(params.Password)
		if err != nil {
			return apierrors.FromErr(http.StatusInternalServerError, err)
		}
		identity, err := q.CreateIdentity(r.Context(), database.CreateIdentityParams{
			Email:        params.Email,
			PasswordHash: passwordHash,
		})
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		u, err := q.CreateUser(r.Context(), database.CreateUserParams{
			ID:        identity.ID,
			FirstName: params.FirstName,
			LastName:  params.LastName,
			Email:     params.Email,
		})
		if err != nil {
			return apierrors.FromDBErr(err)
		}
		user = u
		return nil
	})
	if err != nil {
		s.respondWithError(r.Context(), w, err)
		return
	}

	s.respondWithJSON(w, http.StatusCreated, users.DBToUser(user))
}
