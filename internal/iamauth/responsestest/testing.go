package responsestest

import (
	"strings"

	"github.com/hashicorp/consul/internal/iamauth/responses"
)

func MakeGetCallerIdentityResponse(arn, userId, accountId string) responses.GetCallerIdentityResponse {
	// Sanity check the UserId for unit tests.
	parsed := parseArn(arn)
	switch parsed.Type {
	case "assumed-role":
		if !strings.Contains(userId, ":") {
			panic("UserId for assumed-role in GetCallerIdentity response must be '<uniqueId>:<session>'")
		}
	default:
		if strings.Contains(userId, ":") {
			panic("UserId in GetCallerIdentity must not contain ':'")
		}
	}

	return responses.GetCallerIdentityResponse{
		GetCallerIdentityResult: []responses.GetCallerIdentityResult{
			{
				Arn:     arn,
				UserId:  userId,
				Account: accountId,
			},
		},
	}
}

func MakeGetRoleResponse(arn, id string, tags ...responses.Tag) responses.GetRoleResponse {
	if strings.Contains(id, ":") {
		panic("RoleId in GetRole response must not contain ':'")
	}
	parsed := parseArn(arn)
	return responses.GetRoleResponse{
		GetRoleResult: []responses.GetRoleResult{
			{
				Role: responses.Role{
					Arn:      arn,
					Path:     parsed.Path,
					RoleId:   id,
					RoleName: parsed.FriendlyName,
					Tags:     tags,
				},
			},
		},
	}
}

func MakeGetUserResponse(arn, id string, tags ...responses.Tag) responses.GetUserResponse {
	if strings.Contains(id, ":") {
		panic("UserId in GetUser resposne must not contain ':'")
	}
	parsed := parseArn(arn)
	return responses.GetUserResponse{
		GetUserResult: []responses.GetUserResult{
			{
				User: responses.User{
					Arn:      arn,
					Path:     parsed.Path,
					UserId:   id,
					UserName: parsed.FriendlyName,
					Tags:     tags,
				},
			},
		},
	}
}

func parseArn(arn string) *responses.ParsedArn {
	parsed, err := responses.ParseArn(arn)
	if err != nil {
		// For testing, just fail immediately.
		panic(err)
	}
	return parsed
}
