package responses

import "encoding/xml"

type GetCallerIdentityResponse struct {
	XMLName                 xml.Name                  `xml:"GetCallerIdentityResponse"`
	GetCallerIdentityResult []GetCallerIdentityResult `xml:"GetCallerIdentityResult"`
	ResponseMetadata        []ResponseMetadata        `xml:"ResponseMetadata"`
}

type GetCallerIdentityResult struct {
	Arn     string `xml:"Arn"`
	UserId  string `xml:"UserId"`
	Account string `xml:"Account"`
}

type ResponseMetadata struct {
	RequestId string `xml:"RequestId"`
}

// IAMEntity is an interface for getting details from an IAM Role or User.
type IAMEntity interface {
	EntityPath() string
	EntityArn() string
	EntityName() string
	EntityId() string
	EntityTags() map[string]string
}

var _ IAMEntity = (*Role)(nil)
var _ IAMEntity = (*User)(nil)

type GetRoleResponse struct {
	XMLName          xml.Name           `xml:"GetRoleResponse"`
	GetRoleResult    []GetRoleResult    `xml:"GetRoleResult"`
	ResponseMetadata []ResponseMetadata `xml:"ResponseMetadata"`
}

type GetRoleResult struct {
	Role Role `xml:"Role"`
}

type Role struct {
	Arn      string `xml:"Arn"`
	Path     string `xml:"Path"`
	RoleId   string `xml:"RoleId"`
	RoleName string `xml:"RoleName"`
	Tags     Tags   `xml:"Tags"`
}

func (r *Role) EntityPath() string            { return r.Path }
func (r *Role) EntityArn() string             { return r.Arn }
func (r *Role) EntityName() string            { return r.RoleName }
func (r *Role) EntityId() string              { return r.RoleId }
func (r *Role) EntityTags() map[string]string { return tagsToMap(r.Tags) }

type GetUserResponse struct {
	XMLName          xml.Name           `xml:"GetUserResponse"`
	GetUserResult    []GetUserResult    `xml:"GetUserResult"`
	ResponseMetadata []ResponseMetadata `xml:"ResponseMetadata"`
}

type GetUserResult struct {
	User User `xml:"User"`
}

type User struct {
	Arn      string `xml:"Arn"`
	Path     string `xml:"Path"`
	UserId   string `xml:"UserId"`
	UserName string `xml:"UserName"`
	Tags     Tags   `xml:"Tags"`
}

func (u *User) EntityPath() string            { return u.Path }
func (u *User) EntityArn() string             { return u.Arn }
func (u *User) EntityName() string            { return u.UserName }
func (u *User) EntityId() string              { return u.UserId }
func (u *User) EntityTags() map[string]string { return tagsToMap(u.Tags) }

type Tags struct {
	Members []TagMember `xml:"member"`
}

type TagMember struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

func tagsToMap(tags Tags) map[string]string {
	result := map[string]string{}
	for _, tag := range tags.Members {
		result[tag.Key] = tag.Value
	}
	return result
}
