export interface User {
  user_id: string;
  username: string;
  avatar: string;
}

export interface Guild {
  guild_id: string;
  name: string;
  icon: string | null;
  owner_id?: string;
  is_admin: boolean;
}

export interface Streamer {
  id: string;
  twitch_broadcaster_id: string;
  twitch_login: string;
  twitch_display_name: string;
  twitch_avatar_url: string;
  custom_content?: string;
  added_by?: string;
}

export interface Channel {
  id: string;
  type: number;
  name: string;
  position: number;
}

export interface Role {
  id: string;
  name: string;
  color: number;
  position: number;
}

export interface GuildConfig {
  guild_id: string;
  channel_id: string;
  mention_role_id: string | null;
  message_template: MessageTemplate;
  enabled: boolean;
}

export interface MessageTemplate {
  content: string;
  embed?: {
    title: string;
    description: string;
    url: string;
    color: number;
    thumbnail?: { url: string };
    image?: { url: string };
    fields?: Array<{
      name: string;
      value: string;
      inline: boolean;
    }>;
    footer?: { text: string };
    timestamp?: boolean;
  };
}

export interface UserPreference {
  user_id: string;
  guild_id: string;
  streamer_id: string;
  notifications_enabled: boolean;
}

export interface InviteLink {
  id: string;
  guild_id: string;
  code: string;
  created_by: string;
  expires_at: string | null;
  max_uses: number;
  use_count: number;
  created_at: string;
}

export interface InviteInfo {
  code: string;
  guild_id: string;
  guild_name: string;
  guild_icon: string | null;
  valid: boolean;
}
