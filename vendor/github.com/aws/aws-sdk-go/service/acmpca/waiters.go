// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

package acmpca

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
)

// WaitUntilAuditReportCreated uses the ACM-PCA API operation
// DescribeCertificateAuthorityAuditReport to wait for a condition to be met before returning.
// If the condition is not met within the max attempt window, an error will
// be returned.
func (c *ACMPCA) WaitUntilAuditReportCreated(input *DescribeCertificateAuthorityAuditReportInput) error {
	return c.WaitUntilAuditReportCreatedWithContext(aws.BackgroundContext(), input)
}

// WaitUntilAuditReportCreatedWithContext is an extended version of WaitUntilAuditReportCreated.
// With the support for passing in a context and options to configure the
// Waiter and the underlying request options.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *ACMPCA) WaitUntilAuditReportCreatedWithContext(ctx aws.Context, input *DescribeCertificateAuthorityAuditReportInput, opts ...request.WaiterOption) error {
	w := request.Waiter{
		Name:        "WaitUntilAuditReportCreated",
		MaxAttempts: 60,
		Delay:       request.ConstantWaiterDelay(3 * time.Second),
		Acceptors: []request.WaiterAcceptor{
			{
				State:   request.SuccessWaiterState,
				Matcher: request.PathWaiterMatch, Argument: "AuditReportStatus",
				Expected: "SUCCESS",
			},
			{
				State:   request.FailureWaiterState,
				Matcher: request.PathWaiterMatch, Argument: "AuditReportStatus",
				Expected: "FAILED",
			},
		},
		Logger: c.Config.Logger,
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *DescribeCertificateAuthorityAuditReportInput
			if input != nil {
				tmp := *input
				inCpy = &tmp
			}
			req, _ := c.DescribeCertificateAuthorityAuditReportRequest(inCpy)
			req.SetContext(ctx)
			req.ApplyOptions(opts...)
			return req, nil
		},
	}
	w.ApplyOptions(opts...)

	return w.WaitWithContext(ctx)
}

// WaitUntilCertificateAuthorityCSRCreated uses the ACM-PCA API operation
// GetCertificateAuthorityCsr to wait for a condition to be met before returning.
// If the condition is not met within the max attempt window, an error will
// be returned.
func (c *ACMPCA) WaitUntilCertificateAuthorityCSRCreated(input *GetCertificateAuthorityCsrInput) error {
	return c.WaitUntilCertificateAuthorityCSRCreatedWithContext(aws.BackgroundContext(), input)
}

// WaitUntilCertificateAuthorityCSRCreatedWithContext is an extended version of WaitUntilCertificateAuthorityCSRCreated.
// With the support for passing in a context and options to configure the
// Waiter and the underlying request options.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *ACMPCA) WaitUntilCertificateAuthorityCSRCreatedWithContext(ctx aws.Context, input *GetCertificateAuthorityCsrInput, opts ...request.WaiterOption) error {
	w := request.Waiter{
		Name:        "WaitUntilCertificateAuthorityCSRCreated",
		MaxAttempts: 60,
		Delay:       request.ConstantWaiterDelay(3 * time.Second),
		Acceptors: []request.WaiterAcceptor{
			{
				State:    request.SuccessWaiterState,
				Matcher:  request.StatusWaiterMatch,
				Expected: 200,
			},
			{
				State:    request.RetryWaiterState,
				Matcher:  request.ErrorWaiterMatch,
				Expected: "RequestInProgressException",
			},
		},
		Logger: c.Config.Logger,
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *GetCertificateAuthorityCsrInput
			if input != nil {
				tmp := *input
				inCpy = &tmp
			}
			req, _ := c.GetCertificateAuthorityCsrRequest(inCpy)
			req.SetContext(ctx)
			req.ApplyOptions(opts...)
			return req, nil
		},
	}
	w.ApplyOptions(opts...)

	return w.WaitWithContext(ctx)
}

// WaitUntilCertificateIssued uses the ACM-PCA API operation
// GetCertificate to wait for a condition to be met before returning.
// If the condition is not met within the max attempt window, an error will
// be returned.
func (c *ACMPCA) WaitUntilCertificateIssued(input *GetCertificateInput) error {
	return c.WaitUntilCertificateIssuedWithContext(aws.BackgroundContext(), input)
}

// WaitUntilCertificateIssuedWithContext is an extended version of WaitUntilCertificateIssued.
// With the support for passing in a context and options to configure the
// Waiter and the underlying request options.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
func (c *ACMPCA) WaitUntilCertificateIssuedWithContext(ctx aws.Context, input *GetCertificateInput, opts ...request.WaiterOption) error {
	w := request.Waiter{
		Name:        "WaitUntilCertificateIssued",
		MaxAttempts: 60,
		Delay:       request.ConstantWaiterDelay(3 * time.Second),
		Acceptors: []request.WaiterAcceptor{
			{
				State:    request.SuccessWaiterState,
				Matcher:  request.StatusWaiterMatch,
				Expected: 200,
			},
			{
				State:    request.RetryWaiterState,
				Matcher:  request.ErrorWaiterMatch,
				Expected: "RequestInProgressException",
			},
		},
		Logger: c.Config.Logger,
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *GetCertificateInput
			if input != nil {
				tmp := *input
				inCpy = &tmp
			}
			req, _ := c.GetCertificateRequest(inCpy)
			req.SetContext(ctx)
			req.ApplyOptions(opts...)
			return req, nil
		},
	}
	w.ApplyOptions(opts...)

	return w.WaitWithContext(ctx)
}
