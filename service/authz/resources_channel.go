package authz

const (
	ResourceChannel = "channel"

	ActionRead           = "read"
	ActionOperate        = "operate"
	ActionWrite          = "write"
	ActionSensitiveWrite = "sensitive_write"
	ActionSecretView     = "secret_view"
	ActionTest           = "test"
	ActionNameView       = "name_view"
	ActionBaseURLView    = "base_url_view"
)

var (
	ChannelRead           = Permission{Resource: ResourceChannel, Action: ActionRead}
	ChannelOperate        = Permission{Resource: ResourceChannel, Action: ActionOperate}
	ChannelWrite          = Permission{Resource: ResourceChannel, Action: ActionWrite}
	ChannelSensitiveWrite = Permission{Resource: ResourceChannel, Action: ActionSensitiveWrite}
	ChannelSecretView     = Permission{Resource: ResourceChannel, Action: ActionSecretView}
	ChannelTest           = Permission{Resource: ResourceChannel, Action: ActionTest}
	ChannelNameView       = Permission{Resource: ResourceChannel, Action: ActionNameView}
	ChannelBaseURLView    = Permission{Resource: ResourceChannel, Action: ActionBaseURLView}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceChannel,
		LabelKey: "Channel Management",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read channels",
				DescriptionKey: "View channel lists and details without secrets.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionOperate,
				LabelKey:       "Operate channels",
				DescriptionKey: "Refresh balances and enable/disable individual, batch, or tagged channels.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionTest,
				LabelKey:       "Test channels",
				DescriptionKey: "Run the built-in connectivity test on individual channels or all channels.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionNameView,
				LabelKey:       "View real channel names",
				DescriptionKey: "Without it, channel names are replaced by the alias set by root, or by the channel ID.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
				NotTokenScope:  true,
			},
			{
				Action:         ActionBaseURLView,
				LabelKey:       "View channel base URLs",
				DescriptionKey: "Without it, channel base URLs are hidden in channel lists and details.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
				NotTokenScope:  true,
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Edit channel routing",
				DescriptionKey: "Edit non-sensitive settings such as models, groups, and routing rules.",
				DefaultRoles:   []string{BuiltInRoleAdmin},
			},
			{
				Action:         ActionSensitiveWrite,
				LabelKey:       "Edit sensitive channel settings",
				DescriptionKey: "Create channels or edit keys, base URLs, and overrides.",
			},
			{
				Action:         ActionSecretView,
				LabelKey:       "View channel secrets",
				DescriptionKey: "Reserved for viewing complete channel keys after secure verification.",
			},
		},
	})
}
