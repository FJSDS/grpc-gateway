package runtime

import (
	"io"
	"net/http"

	"github.com/golang/protobuf/proto"
	google_protobuf "github.com/golang/protobuf/ptypes/any"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"
)

//HTTPError desc http error status

const (
	//OK No error.
	OK = "OK"

	//InvalidArgument Client specified an invalid argument. Check error message and error details for more information.
	InvalidArgument = "INVALID_ARGUMENT"

	//FailedPrecondition Request can not be executed in the current system state, such as deleting a non-empty directory.
	FailedPrecondition = "FAILED_PRECONDITION"

	//OutOfRange Client specified an invalid range.
	OutOfRange = "OUT_OF_RANGE"

	//Unauthenticated Request not authenticated due to missing, invalid, or expired OAuth token.
	Unauthenticated = "UNAUTHENTICATED"

	//PermissionDenied Client does not have sufficient permission. This can happen because the OAuth token does not have the right scopes,
	// the client doesn't have permission, or the API has not been enabled for the client project.
	PermissionDenied = "PERMISSION_DENIED"

	//NotFound A specified resource is not found, or the request is rejected by undisclosed reasons, such as whitelisting.
	NotFound = "NOT_FOUND"

	//Aborted Concurrency conflict, such as read-modify-write conflict.
	Aborted = "ABORTED"

	//AlreadyExists The resource that a client tried to create already exists.
	AlreadyExists = "ALREADY_EXISTS"

	//ResourceExhausted Either out of resource quota or reaching rate limiting.
	ResourceExhausted = "RESOURCE_EXHAUSTED"

	//Cancelled Request cancelled by the client.
	Canceled = "CANCELLED"

	//DataLoss Unrecoverable data loss or data corruption. The client should report the error to the user.
	DataLoss = "DATA_LOSS"

	//Unknown Unknown server error. Typically a server bug.
	Unknown = "UNKNOWN"

	//Internal Internal server error. Typically a server bug.
	Internal = "INTERNAL"

	//NotImplemented API method not implemented by the server.
	NotImplemented = "NOT_IMPLEMENTED"

	//Unavailable Service unavailable. Typically the server is down.
	Unavailable = "UNAVAILABLE"

	//DeadlineExceeded Request deadline exceeded. This will happen only if the caller sets a deadline that is shorter than the method's default deadline
	// (i.e. requested deadline is not enough for the server to process the request) and the request did not finish within the deadline.
	DeadlineExceeded = "DEADLINE_EXCEEDED"
)

func HTTPStatusStringFromCode(code codes.Code) string {
	switch code {
	case codes.OK:
		return OK
	case codes.Canceled:
		return Canceled
	case codes.Unknown:
		return Unknown
	case codes.InvalidArgument:
		return InvalidArgument
	case codes.DeadlineExceeded:
		return DeadlineExceeded
	case codes.NotFound:
		return NotFound
	case codes.AlreadyExists:
		return AlreadyExists
	case codes.PermissionDenied:
		return PermissionDenied
	case codes.Unauthenticated:
		return Unauthenticated
	case codes.ResourceExhausted:
		return ResourceExhausted
	case codes.FailedPrecondition:
		return FailedPrecondition
	case codes.Aborted:
		return Aborted
	case codes.OutOfRange:
		return OutOfRange
	case codes.Unimplemented:
		return NotImplemented
	case codes.Internal:
		return Internal
	case codes.Unavailable:
		return Unavailable
	case codes.DataLoss:
		return DataLoss
	}

	grpclog.Printf("Unknown gRPC error code: %v", code)
	return Internal
}

// HTTPStatusFromCode converts a gRPC error code into the corresponding HTTP response status.
func HTTPStatusFromCode(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusRequestTimeout
	case codes.Unknown:
		return http.StatusInternalServerError
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusRequestTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.ResourceExhausted:
		return http.StatusForbidden
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	}

	grpclog.Printf("Unknown gRPC error code: %v", code)
	return http.StatusInternalServerError
}

var (
	// HTTPError replies to the request with the error.
	// You can set a custom function to this variable to customize error format.
	HTTPError = DefaultHTTPError
	// OtherErrorHandler handles the following error used by the gateway: StatusMethodNotAllowed StatusNotFound and StatusBadRequest
	OtherErrorHandler = DefaultOtherErrorHandler
)

type errorBody struct {
	Error *errorInfo `protobuf:"bytes,1,name=error" json:"error"`
}

// Make this also conform to proto.Message for builtin JSONPb Marshaler
func (e *errorBody) Reset()         { *e = errorBody{} }
func (e *errorBody) String() string { return proto.CompactTextString(e) }
func (*errorBody) ProtoMessage()    {}

type errorInfo struct {
	Code    int32                  `protobuf:"varint,1,name=code" json:"code"`
	Message string                 `protobuf:"bytes,2,name=message" json:"message,omitempty"`
	Status  string                 `protobuf:"bytes,3,name=status" json:"status"`
	Details []*google_protobuf.Any `protobuf:"bytes,4,rep,name=details" json:"details,omitempty"`
}

//Make this also conform to proto.Message for builtin JSONPb Marshaler
func (e *errorInfo) Reset()         { *e = errorInfo{} }
func (e *errorInfo) String() string { return proto.CompactTextString(e) }
func (*errorInfo) ProtoMessage()    {}

// DefaultHTTPError is the default implementation of HTTPError.
// If "err" is an error from gRPC system, the function replies with the status code mapped by HTTPStatusFromCode.
// If otherwise, it replies with http.StatusInternalServerError.
//
// The response body returned by this function is a JSON object,
// which contains a member whose key is "error" and whose value is err.Error().
func DefaultHTTPError(ctx context.Context, mux *ServeMux, marshaler Marshaler, w http.ResponseWriter, _ *http.Request, err error) {
	const fallback = `{"error": "failed to marshal error message"}`

	w.Header().Del("Trailer")
	w.Header().Set("Content-Type", marshaler.ContentType())

	s, ok := status.FromError(err)
	if !ok {
		s = status.New(codes.Unknown, err.Error())
	}
	sp := s.Proto()
	body := &errorBody{
		Error: &errorInfo{
			Code:    int32(sp.Code),
			Message: sp.Message,
			Details: sp.Details,
			Status:  HTTPStatusStringFromCode(s.Code()),
		},
	}

	buf, merr := marshaler.Marshal(body)
	if merr != nil {
		grpclog.Printf("Failed to marshal error message %q: %v", body, merr)
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := io.WriteString(w, fallback); err != nil {
			grpclog.Printf("Failed to write response: %v", err)
		}
		return
	}

	md, ok := ServerMetadataFromContext(ctx)
	if !ok {
		grpclog.Printf("Failed to extract ServerMetadata from context")
	}

	handleForwardResponseServerMetadata(w, mux, md)
	handleForwardResponseTrailerHeader(w, md)
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(buf); err != nil {
		grpclog.Printf("Failed to write response: %v", err)
	}

	handleForwardResponseTrailer(w, md)
}

// DefaultOtherErrorHandler is the default implementation of OtherErrorHandler.
// It simply writes a string representation of the given error into "w".
func DefaultOtherErrorHandler(w http.ResponseWriter, _ *http.Request, msg string, code int) {
	http.Error(w, msg, code)
}
