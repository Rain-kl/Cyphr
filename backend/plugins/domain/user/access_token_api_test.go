// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user_test

import (
	"Wavelet/core/contracts"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type createTokenResponseEnvelope struct {
	ErrorMsg string `json:"error_msg"`
	Data     struct {
		Token    string          `json:"token"`
		RawToken string          `json:"raw_token"`
		Record   json.RawMessage `json:"record"`
	} `json:"data"`
}

type tokenRecordDTO struct {
	ID          string `json:"id"`
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	MaskedToken string `json:"masked_token"`
	IsAdmin     bool   `json:"is_admin"`
}

func TestAccessTokenAPILifecycle(t *testing.T) {
	engine, userSvc := mountUserAuthEngine(t)
	bg := context.Background()

	u, err := userSvc.CreateUser(bg, contracts.CreateUserRequest{
		Username: "token_user",
		Password: "Password123!",
		Email:    "token_user@example.com",
		IsAdmin:  false,
	})
	require.NoError(t, err)

	cookies := loginAndCookie(t, engine, u.Username, "Password123!")

	// 1. Create Access Token
	createBody := `{"name":"test-api-key","is_admin":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/user/access-tokens", bytes.NewBufferString(createBody))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "create token response: %s", rec.Body.String())

	var createEnv createTokenResponseEnvelope
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &createEnv))
	require.Empty(t, createEnv.ErrorMsg)
	require.True(t, strings.HasPrefix(createEnv.Data.Token, "wvt_"), "token should be plaintext wvt_... string, got: %s", createEnv.Data.Token)
	require.Equal(t, createEnv.Data.Token, createEnv.Data.RawToken)

	var record tokenRecordDTO
	require.NoError(t, json.Unmarshal(createEnv.Data.Record, &record))
	require.NotEmpty(t, record.ID, "token ID must not be empty or 0")
	require.NotEqual(t, "0", record.ID, "token ID must be a generated snowflake ID, not 0")
	tokenID := record.ID

	// 2. List Access Tokens
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/user/access-tokens", nil)
	for _, c := range cookies {
		listReq.AddCookie(c)
	}
	listRec := httptest.NewRecorder()
	engine.ServeHTTP(listRec, listReq)

	require.Equal(t, http.StatusOK, listRec.Code)
	var listEnv struct {
		ErrorMsg string           `json:"error_msg"`
		Data     []tokenRecordDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRec.Body.Bytes(), &listEnv))
	require.Empty(t, listEnv.ErrorMsg)
	require.Len(t, listEnv.Data, 1)
	require.Equal(t, tokenID, listEnv.Data[0].ID)
	require.Equal(t, "test-api-key", listEnv.Data[0].Name)

	// 3. Rotate Access Token using the token ID
	rotateReq := httptest.NewRequest(http.MethodPost, "/api/v1/user/access-tokens/"+tokenID+"/rotate", nil)
	for _, c := range cookies {
		rotateReq.AddCookie(c)
	}
	rotateRec := httptest.NewRecorder()
	engine.ServeHTTP(rotateRec, rotateReq)

	require.Equal(t, http.StatusOK, rotateRec.Code, "rotate token response: %s", rotateRec.Body.String())
	var rotateEnv createTokenResponseEnvelope
	require.NoError(t, json.Unmarshal(rotateRec.Body.Bytes(), &rotateEnv))
	require.Empty(t, rotateEnv.ErrorMsg)
	require.True(t, strings.HasPrefix(rotateEnv.Data.Token, "wvt_"))
	require.NotEqual(t, createEnv.Data.Token, rotateEnv.Data.Token, "rotated token should generate a new secret")

	// 4. Delete Access Token
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/user/access-tokens/"+tokenID, nil)
	for _, c := range cookies {
		deleteReq.AddCookie(c)
	}
	deleteRec := httptest.NewRecorder()
	engine.ServeHTTP(deleteRec, deleteReq)
	require.Equal(t, http.StatusOK, deleteRec.Code)

	// 5. Verify Token was deleted
	listReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/user/access-tokens", nil)
	for _, c := range cookies {
		listReq2.AddCookie(c)
	}
	listRec2 := httptest.NewRecorder()
	engine.ServeHTTP(listRec2, listReq2)
	var listEnv2 struct {
		ErrorMsg string           `json:"error_msg"`
		Data     []tokenRecordDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(listRec2.Body.Bytes(), &listEnv2))
	require.Empty(t, listEnv2.Data)
}
