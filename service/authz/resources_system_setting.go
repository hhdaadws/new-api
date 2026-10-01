package authz

const ResourceSystemSetting = "system_setting"

var (
	SystemSettingRead  = Permission{Resource: ResourceSystemSetting, Action: ActionRead}
	SystemSettingWrite = Permission{Resource: ResourceSystemSetting, Action: ActionWrite}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceSystemSetting,
		LabelKey: "System Settings",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "View system settings",
				DescriptionKey: "Open the system settings pages. Secrets are never shown.",
				NotTokenScope:  true,
			},
			{
				Action:         ActionWrite,
				LabelKey:       "Edit system settings",
				DescriptionKey: "Change system settings. Secrets, payment settings, OAuth providers, and maintenance tools remain root-only.",
				NotTokenScope:  true,
			},
		},
	})
}
