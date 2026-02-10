import type { Guild, Channel, Role, Streamer, GuildConfig, UserPreference, User, InviteLink, InviteInfo } from '../types';

// In production VITE_API_URL is "" (same origin via CloudFront).
// Use ?? so empty string isn't treated as missing (|| would fall back to localhost).
const API_BASE = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

async function fetchAPI<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  if (!response.ok) {
    if (response.status === 401) {
      throw new Error('Unauthorized');
    }
    const text = await response.text();
    throw new Error(`API error (${response.status}): ${text}`);
  }

  return response.json();
}

// Auth
export function loginWithDiscord() {
  window.location.href = `${API_BASE}/api/auth/discord/login`;
}

export async function logout(): Promise<{ message: string }> {
  return fetchAPI('/api/auth/logout', { method: 'POST' });
}

export async function getMe(): Promise<User> {
  return fetchAPI('/api/auth/me');
}

// Guilds
export async function getUserGuilds(): Promise<Guild[]> {
  return fetchAPI('/api/guilds');
}

export async function getGuildChannels(guildId: string): Promise<Channel[]> {
  return fetchAPI(`/api/guilds/${guildId}/channels`);
}

export async function getGuildRoles(guildId: string): Promise<Role[]> {
  return fetchAPI(`/api/guilds/${guildId}/roles`);
}

export async function getGuildStreamers(guildId: string): Promise<Streamer[]> {
  return fetchAPI(`/api/guilds/${guildId}/streamers`);
}

export async function initiateStreamerLink(guildId: string): Promise<{ url: string }> {
  return fetchAPI(`/api/guilds/${guildId}/streamers/link`);
}

export async function unlinkStreamer(guildId: string, streamerId: string): Promise<{ message: string }> {
  return fetchAPI(`/api/guilds/${guildId}/streamers/${streamerId}`, {
    method: 'DELETE',
  });
}

export async function getGuildConfig(guildId: string): Promise<GuildConfig> {
  return fetchAPI(`/api/guilds/${guildId}/config`);
}

export async function updateGuildConfig(guildId: string, config: Partial<GuildConfig>): Promise<{ message: string }> {
  return fetchAPI(`/api/guilds/${guildId}/config`, {
    method: 'PUT',
    body: JSON.stringify(config),
  });
}

export async function getBotInstallURL(guildId: string): Promise<{ url: string }> {
  return fetchAPI(`/api/guilds/${guildId}/bot-install-url`);
}

// Streamer message (custom notification text)
export async function getStreamerMessage(guildId: string, streamerId: string): Promise<{ custom_content: string }> {
  return fetchAPI(`/api/guilds/${guildId}/streamers/${streamerId}/message`);
}

export async function updateStreamerMessage(guildId: string, streamerId: string, customContent: string): Promise<{ message: string }> {
  return fetchAPI(`/api/guilds/${guildId}/streamers/${streamerId}/message`, {
    method: 'PUT',
    body: JSON.stringify({ custom_content: customContent }),
  });
}

// Invite links
export async function createInviteLink(guildId: string, expiresInHours = 0, maxUses = 0): Promise<InviteLink> {
  return fetchAPI(`/api/guilds/${guildId}/invites`, {
    method: 'POST',
    body: JSON.stringify({ expires_in_hours: expiresInHours, max_uses: maxUses }),
  });
}

export async function getInviteLinks(guildId: string): Promise<InviteLink[]> {
  return fetchAPI(`/api/guilds/${guildId}/invites`);
}

export async function deleteInviteLink(guildId: string, inviteId: string): Promise<{ message: string }> {
  return fetchAPI(`/api/guilds/${guildId}/invites/${inviteId}`, {
    method: 'DELETE',
  });
}

export async function getInviteInfo(code: string): Promise<InviteInfo> {
  return fetchAPI(`/api/invites/${code}`);
}

export async function acceptInvite(code: string): Promise<{ message: string; guild_id: string; name: string }> {
  return fetchAPI(`/api/invites/${code}/accept`, { method: 'POST' });
}

// User Preferences
export async function getUserPreferences(): Promise<UserPreference[]> {
  return fetchAPI('/api/users/me/preferences');
}

export async function updateUserPreference(
  guildId: string,
  streamerId: string,
  enabled: boolean
): Promise<{ message: string }> {
  return fetchAPI(`/api/users/me/preferences/${guildId}/${streamerId}`, {
    method: 'PUT',
    body: JSON.stringify({ enabled }),
  });
}
