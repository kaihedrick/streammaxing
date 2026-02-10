package discord

const (
	// PermissionAdministrator is the Discord ADMINISTRATOR permission bit
	PermissionAdministrator int64 = 0x8
	// PermissionManageGuild is the MANAGE_GUILD permission bit
	PermissionManageGuild int64 = 0x20
)

// HasAdminPermission checks if the permissions integer includes ADMINISTRATOR
func HasAdminPermission(permissions int64) bool {
	return (permissions & PermissionAdministrator) != 0
}

// HasManageGuildPermission checks if the permissions include MANAGE_GUILD
func HasManageGuildPermission(permissions int64) bool {
	return (permissions&PermissionAdministrator) != 0 || (permissions&PermissionManageGuild) != 0
}

// IsAdminInGuild checks if a user has admin permission in a specific guild
func IsAdminInGuild(guilds []DiscordGuild, guildID string) bool {
	for _, guild := range guilds {
		if guild.ID == guildID {
			return guild.Owner || HasAdminPermission(guild.Permissions)
		}
	}
	return false
}
