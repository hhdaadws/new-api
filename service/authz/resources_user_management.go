package authz

const ResourceUserManagement = "user_management"

var (
	UserManagementRead  = Permission{Resource: ResourceUserManagement, Action: ActionRead}
	UserManagementWrite = Permission{Resource: ResourceUserManagement, Action: ActionWrite}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceUserManagement,
		LabelKey: "User Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "View users",
				DescriptionKey: "View the user list, user details, and sign-in bindings.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
				NotTokenScope:  true,
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Manage users",
				DescriptionKey: "Create, edit, disable, and delete users, and reset their security settings.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
				NotTokenScope:  true,
			},
		},
	})
}
