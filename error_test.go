package fauna

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetErrFauna(t *testing.T) {
	type args struct {
		httpStatus   int
		serviceError *ErrFauna
		errType      error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "No error",
			args: args{
				httpStatus:   200,
				serviceError: nil,
				errType:      nil,
			},
			wantErr: false,
		},
		{
			name: "Query check error",
			args: args{
				httpStatus:   http.StatusBadRequest,
				serviceError: &ErrFauna{Code: "invalid_query", Message: ""},
				errType:      &ErrQueryCheck{},
			},
			wantErr: true,
		},
		{
			name: "Query runtime error",
			args: args{
				httpStatus:   http.StatusBadRequest,
				serviceError: &ErrFauna{Code: "invalid_argument", Message: ""},
				errType:      &ErrQueryRuntime{},
			},
			wantErr: true,
		},
		{
			name: "Invalid request error",
			args: args{
				httpStatus:   http.StatusBadRequest,
				serviceError: &ErrFauna{Code: "invalid_request", Message: ""},
				errType:      &ErrInvalidRequest{},
			},
			wantErr: true,
		},
		{
			name: "Abort error",
			args: args{
				httpStatus:   http.StatusBadRequest,
				serviceError: &ErrFauna{Code: "abort", Message: "", Abort: `{"@int":"1234"}`},
				errType:      &ErrAbort{},
			},
			wantErr: true,
		},
		{
			name: "Unauthorized",
			args: args{
				httpStatus:   http.StatusUnauthorized,
				serviceError: &ErrFauna{Code: "", Message: ""},
				errType:      &ErrAuthentication{},
			},
			wantErr: true,
		},
		{
			name: "Access not granted",
			args: args{
				httpStatus:   http.StatusForbidden,
				serviceError: &ErrFauna{Code: "", Message: ""},
				errType:      &ErrAuthorization{},
			},
			wantErr: true,
		},
		{
			name: "Too many requests",
			args: args{
				httpStatus:   http.StatusTooManyRequests,
				serviceError: &ErrFauna{Code: "", Message: ""},
				errType:      &ErrThrottling{},
			},
			wantErr: true,
		},
		{
			name: "Query timeout",
			args: args{
				httpStatus:   440,
				serviceError: &ErrFauna{Code: "", Message: ""},
				errType:      &ErrQueryTimeout{},
			},
			wantErr: true,
		},
		{
			name: "Internal error",
			args: args{
				httpStatus:   http.StatusInternalServerError,
				serviceError: &ErrFauna{Code: "", Message: ""},
				errType:      &ErrServiceInternal{},
			},
			wantErr: true,
		},
		{
			name: "Service timeout",
			args: args{
				httpStatus:   http.StatusServiceUnavailable,
				serviceError: &ErrFauna{Code: "", Message: ""},
				errType:      &ErrServiceTimeout{},
			},
			wantErr: true,
		},
		{
			name: "Contended transaction",
			args: args{
				httpStatus:   http.StatusConflict,
				serviceError: &ErrFauna{Code: "contended_transaction", Message: "Transaction was aborted due to detection of concurrent modification."},
				errType:      &ErrContendedTransaction{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := &queryResponse{Error: tt.args.serviceError, Summary: ""}
			err := getErrFauna(tt.args.httpStatus, res)
			if tt.wantErr {
				assert.ErrorAs(t, err, &tt.args.errType)
				assert.NotZero(t, res.Error.StatusCode)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestErrAbort(t *testing.T) {
	t.Setenv(EnvFaunaEndpoint, EndpointLocal)
	t.Setenv(EnvFaunaSecret, "secret")

	client, clientErr := NewDefaultClient()
	if !assert.NoError(t, clientErr) {
		return
	}

	t.Run("abort field can have value", func(t *testing.T) {
		query, _ := FQL(`abort("foo")`, nil)
		_, qErr := client.Query(query)
		var expectedErr *ErrAbort
		if assert.ErrorAs(t, qErr, &expectedErr) {
			assert.Equal(t, "abort", expectedErr.Code)
			assert.Equal(t, "foo", expectedErr.Abort)
		}
	})

	t.Run("ErrAbort handles object and allows unmarshalling", func(t *testing.T) {
		query, _ := FQL(`abort({ msg: "abrasive message", aborted_at: Time("2023-02-28T18:10:10.00001Z")})`, nil)
		_, qErr := client.Query(query)

		type CustomAbort struct {
			Message   string    `fauna:"msg"`
			AbortedAt time.Time `fauna:"aborted_at"`
		}

		var customAbort CustomAbort

		var expectedErr *ErrAbort
		if assert.ErrorAs(t, qErr, &expectedErr) {
			assert.Equal(t, "abort", expectedErr.Code)
			err := expectedErr.Unmarshal(&customAbort)
			assert.NoError(t, err)
			assert.Equal(t, customAbort.AbortedAt, time.Date(2023, 02, 28, 18, 10, 10, 10000, time.UTC))
			assert.Equal(t, customAbort.Message, "abrasive message")
		}
	})
}

func TestErrConstraint(t *testing.T) {
	t.Setenv(EnvFaunaEndpoint, EndpointLocal)
	t.Setenv(EnvFaunaSecret, "secret")

	client, clientErr := NewDefaultClient()
	if !assert.NoError(t, clientErr) {
		return
	}

	t.Run("constraint failures get decoded", func(t *testing.T) {
		retried := false
		query, queryErr := FQL(`Function.create({"name": "double", "body": "x => x * 2"})`, nil)
		if !assert.NoError(t, queryErr) {
			t.FailNow()
		}
	CreateFunction:
		_, qErr := client.Query(query)
		if qErr == nil {
			if !retried {
				// now we try to create the function again
				retried = true
				goto CreateFunction
			}

			// if we retried already and got another error, fail
			t.FailNow()
		}

		var expectedErr *ErrQueryRuntime
		if assert.ErrorAs(t, qErr, &expectedErr) {
			assert.Len(t, expectedErr.ConstraintFailures, 1)
			assert.NotEmpty(t, expectedErr.ConstraintFailures[0].Message)
		}
	})
}
