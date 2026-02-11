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

/**
 * Initiates the Discord OAuth flow from the frontend.
 *
 * Instead of redirecting to the backend (which sets a cookie for CSRF state),
 * the frontend generates the state, stores it in localStorage, and redirects
 * directly to Discord. This avoids the cookie-based state verification that
 * fails in privacy-focused browsers (Brave, etc.) which block cookies on
 * cross-site redirects.
 */
export function loginWithDiscord() {
  const clientId = import.meta.env.VITE_DISCORD_CLIENT_ID;
  if (!clientId) {
    console.error('VITE_DISCORD_CLIENT_ID is not configured');
    return;
  }

  // Generate random state for CSRF protection and store in localStorage
  const state = crypto.randomUUID().replace(/-/g, '');
  localStorage.setItem('oauth_state', state);

  const redirectUri = `${window.location.origin}/auth/callback`;
  const params = new URLSearchParams({
    client_id: clientId,
    redirect_uri: redirectUri,
    response_type: 'code',
    scope: 'identify guilds',
    state,
  });

  window.location.href = `https://discord.com/oauth2/authorize?${params.toString()}`;
}

/**
 * Exchange a Discord authorization code for a session.
 * The backend exchanges the code with Discord, creates/updates the user,
 * sets the session cookie, and returns user info.
 */
export async function exchangeDiscordCode(code: string, redirectUri: string): Promise<User> {
  return fetchAPI('/api/auth/discord/exchange', {
    method: 'POST',
    body: JSON.stringify({ code, redirect_uri: redirectUri }),
  });
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
