// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package consts

import "errors"

// Sentinel errors for data access and domain logic.
var (
	ErrRecordNotFound = errors.New("errRecordNotFound")
	ErrModelNotFound  = errors.New("errModelNotFound")
	ErrNodeNotFound   = errors.New("errNodeNotFound")
	ErrJobNotFound    = errors.New("errJobNotFound")
)

// CamelCase string error codes returned to API clients.
const (
	ErrBindParamsFailed     = "errBindParamsFailed"
	ErrInvalidToken         = "errInvalidToken"
	ErrNodeOffline          = "errNodeOffline"
	ErrNodeInactive         = "errNodeInactive"
	ErrUnauthorized         = "errUnauthorized"
	ErrInternal             = "errInternal"
	ErrInvalidStatus        = "errInvalidStatus"
	ErrModelUnavailable     = "errModelUnavailable"
	ErrFileUploadFailed     = "errFileUploadFailed"
	ErrMediaNotFound        = "errMediaNotFound"
	ErrStreamingUnsupported = "errStreamingUnsupported"
	ErrForbidden            = "errForbidden"
)
