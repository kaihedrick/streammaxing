# Task 005: Admin Dashboard

## Status
Complete

## Overview
Build the frontend admin dashboard and user settings interface for managing guilds, streamers, notification configuration, and user preferences.

---

## Goals
1. Create landing page with Discord login
2. Implement dashboard layout with navigation
3. Build guild selection and management interface
4. Create streamer management UI (add/remove streamers)
5. Build guild configuration editor (channel, role, template)
6. Implement template editor with live preview
7. Create user settings page for notification preferences
8. Implement error handling and loading states
9. Add responsive design for mobile devices
10. Deploy to S3 + CloudFront

---

## Prerequisites
- Task 001 (Project Bootstrap) completed
- Task 002 (Discord Auth) completed
- Task 003 (Twitch EventSub) completed
- Task 004 (Notification Fanout) completed
- Frontend initialized with React + TypeScript + Vite
- API endpoints functional

---

## Frontend Architecture

```
frontend/
├── src/
│   ├── components/
│   │   ├── Auth/
│   │   │   ├── LoginPage.tsx
│   │   │   └── ProtectedRoute.tsx
│   │   ├── Dashboard/
│   │   │   ├── DashboardLayout.tsx
│   │   │   ├── GuildSelector.tsx
│   │   │   ├── GuildOverview.tsx
│   │   │   ├── StreamerList.tsx
│   │   │   ├── StreamerCard.tsx
│   │   │   └── AddStreamerButton.tsx
│   │   ├── Config/
│   │   │   ├── GuildConfigEditor.tsx
│   │   │   ├── ChannelSelector.tsx
│   │   │   ├── RoleSelector.tsx
│   │   │   └── TemplateEditor.tsx
│   │   ├── Settings/
│   │   │   ├── UserSettings.tsx
│   │   │   └── NotificationPreferences.tsx
│   │   └── common/
│   │       ├── LoadingSpinner.tsx
│   │       ├── ErrorMessage.tsx
│   │       └── Button.tsx
│   ├── hooks/
│   │   ├── useAuth.ts
│   │   ├── useGuilds.ts
│   │   └── useStreamers.ts
│   ├── services/
│   │   └── api.ts
│   ├── types/
│   │   └── index.ts
│   ├── App.tsx
│   ├── main.tsx
│   └── index.css
```

---

## Implementation

### 1. Types

**File**: `frontend/src/types/index.ts`

```typescript
export interface User {
  user_id: string;
  username: string;
  avatar: string;
}

export interface Guild {
  guild_id: string;
  name: string;
  icon: string | null;
}

export interface Streamer {
  id: string;
  twitch_broadcaster_id: string;
  twitch_login: string;
  twitch_display_name: string;
  twitch_avatar_url: string;
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
  guild_id: string;
  streamer_id: string;
  notifications_enabled: boolean;
}
```

### 2. API Service

**File**: `frontend/src/services/api.ts`

```typescript
const API_BASE = import.meta.env.VITE_API_URL;

async function fetchAPI(path: string, options: RequestInit = {}) {
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
  });

  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }

  return response.json();
}

export async function loginWithDiscord() {
  window.location.href = `${API_BASE}/api/auth/discord/login`;
}

export async function logout() {
  return fetchAPI('/api/auth/logout', { method: 'POST' });
}

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

export async function unlinkStreamer(guildId: string, streamerId: string) {
  return fetchAPI(`/api/guilds/${guildId}/streamers/${streamerId}`, {
    method: 'DELETE',
  });
}

export async function getGuildConfig(guildId: string): Promise<GuildConfig> {
  return fetchAPI(`/api/guilds/${guildId}/config`);
}

export async function updateGuildConfig(guildId: string, config: Partial<GuildConfig>) {
  return fetchAPI(`/api/guilds/${guildId}/config`, {
    method: 'PUT',
    body: JSON.stringify(config),
  });
}

export async function getUserPreferences(): Promise<UserPreference[]> {
  return fetchAPI('/api/users/me/preferences');
}

export async function updateUserPreference(
  guildId: string,
  streamerId: string,
  enabled: boolean
) {
  return fetchAPI(`/api/users/me/preferences/${guildId}/${streamerId}`, {
    method: 'PUT',
    body: JSON.stringify({ enabled }),
  });
}
```

### 3. Auth Hook

**File**: `frontend/src/hooks/useAuth.ts`

```typescript
import { useState, useEffect } from 'react';
import { getUserGuilds } from '../services/api';

export function useAuth() {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    getUserGuilds()
      .then(() => {
        setIsAuthenticated(true);
        setIsLoading(false);
      })
      .catch(() => {
        setIsAuthenticated(false);
        setIsLoading(false);
      });
  }, []);

  return { isAuthenticated, isLoading };
}
```

### 4. Protected Route

**File**: `frontend/src/components/Auth/ProtectedRoute.tsx`

```tsx
import { Navigate } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import { LoadingSpinner } from '../common/LoadingSpinner';

export function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return <LoadingSpinner />;
  }

  if (!isAuthenticated) {
    return <Navigate to="/" replace />;
  }

  return <>{children}</>;
}
```

### 5. Login Page

**File**: `frontend/src/components/Auth/LoginPage.tsx`

```tsx
import { loginWithDiscord } from '../../services/api';
import './LoginPage.css';

export function LoginPage() {
  return (
    <div className="login-page">
      <div className="login-container">
        <h1>StreamMaxing</h1>
        <p>Get notified when your favorite Twitch streamers go live</p>
        <button onClick={loginWithDiscord} className="discord-button">
          <DiscordIcon />
          Login with Discord
        </button>
      </div>
    </div>
  );
}

function DiscordIcon() {
  return (
    <svg width="24" height="24" viewBox="0 0 71 55" fill="currentColor">
      <path d="M60.1045 4.8978C55.5792 2.8214 50.7265 1.2916 45.6527 0.41542C45.5603 0.39851 45.468 0.440769 45.4204 0.525289C44.7963 1.6353 44.105 3.0834 43.6209 4.2216C38.1637 3.4046 32.7345 3.4046 27.3892 4.2216C26.905 3.0581 26.1886 1.6353 25.5617 0.525289C25.5141 0.443589 25.4218 0.40133 25.3294 0.41542C20.2584 1.2888 15.4057 2.8186 10.8776 4.8978C10.8384 4.9147 10.8048 4.9429 10.7825 4.9795C1.57795 18.7309 -0.943561 32.1443 0.293408 45.3914C0.299005 45.4562 0.335386 45.5182 0.385761 45.5576C6.45866 50.0174 12.3413 52.7249 18.1147 54.5195C18.2071 54.5477 18.305 54.5139 18.3638 54.4378C19.7295 52.5728 20.9469 50.6063 21.9907 48.5383C22.0523 48.4172 21.9935 48.2735 21.8676 48.2256C19.9366 47.4931 18.0979 46.6 16.3292 45.5858C16.1893 45.5041 16.1781 45.304 16.3068 45.2082C16.679 44.9293 17.0513 44.6391 17.4067 44.3461C17.471 44.2926 17.5606 44.2813 17.6362 44.3151C29.2558 49.6202 41.8354 49.6202 53.3179 44.3151C53.3935 44.2785 53.4831 44.2898 53.5502 44.3433C53.9057 44.6363 54.2779 44.9293 54.6529 45.2082C54.7816 45.304 54.7732 45.5041 54.6333 45.5858C52.8646 46.6197 51.0259 47.4931 49.0921 48.2228C48.9662 48.2707 48.9102 48.4172 48.9718 48.5383C50.038 50.6034 51.2554 52.5699 52.5959 54.435C52.6519 54.5139 52.7526 54.5477 52.845 54.5195C58.6464 52.7249 64.529 50.0174 70.6019 45.5576C70.6551 45.5182 70.6887 45.459 70.6943 45.3942C72.1747 30.0791 68.2147 16.7757 60.1968 4.9823C60.1772 4.9429 60.1437 4.9147 60.1045 4.8978ZM23.7259 37.3253C20.2276 37.3253 17.3451 34.1136 17.3451 30.1693C17.3451 26.225 20.1717 23.0133 23.7259 23.0133C27.308 23.0133 30.1626 26.2532 30.1066 30.1693C30.1066 34.1136 27.28 37.3253 23.7259 37.3253ZM47.3178 37.3253C43.8196 37.3253 40.9371 34.1136 40.9371 30.1693C40.9371 26.225 43.7636 23.0133 47.3178 23.0133C50.9 23.0133 53.7545 26.2532 53.6986 30.1693C53.6986 34.1136 50.9 37.3253 47.3178 37.3253Z" />
    </svg>
  );
}
```

### 6. Dashboard Layout

**File**: `frontend/src/components/Dashboard/DashboardLayout.tsx`

```tsx
import { useState } from 'react';
import { Outlet, Link, useNavigate } from 'react-router-dom';
import { logout } from '../../services/api';
import './DashboardLayout.css';

export function DashboardLayout() {
  const navigate = useNavigate();

  const handleLogout = async () => {
    await logout();
    navigate('/');
  };

  return (
    <div className="dashboard-layout">
      <header className="dashboard-header">
        <h1>StreamMaxing</h1>
        <nav>
          <Link to="/dashboard">Guilds</Link>
          <Link to="/settings">Settings</Link>
          <button onClick={handleLogout}>Logout</button>
        </nav>
      </header>
      <main className="dashboard-content">
        <Outlet />
      </main>
    </div>
  );
}
```

### 7. Guild Selector

**File**: `frontend/src/components/Dashboard/GuildSelector.tsx`

```tsx
import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getUserGuilds } from '../../services/api';
import { Guild } from '../../types';
import { LoadingSpinner } from '../common/LoadingSpinner';
import { ErrorMessage } from '../common/ErrorMessage';
import './GuildSelector.css';

export function GuildSelector() {
  const [guilds, setGuilds] = useState<Guild[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    getUserGuilds()
      .then(setGuilds)
      .catch(() => setError('Failed to load guilds'))
      .finally(() => setLoading(false));
  }, []);

  if (loading) return <LoadingSpinner />;
  if (error) return <ErrorMessage message={error} />;

  return (
    <div className="guild-selector">
      <h2>Select a Server</h2>
      <div className="guild-grid">
        {guilds.map((guild) => (
          <div
            key={guild.guild_id}
            className="guild-card"
            onClick={() => navigate(`/dashboard/guilds/${guild.guild_id}`)}
          >
            {guild.icon ? (
              <img
                src={`https://cdn.discordapp.com/icons/${guild.guild_id}/${guild.icon}.png`}
                alt={guild.name}
              />
            ) : (
              <div className="guild-icon-placeholder">{guild.name[0]}</div>
            )}
            <h3>{guild.name}</h3>
          </div>
        ))}
      </div>
    </div>
  );
}
```

### 8. Guild Overview

**File**: `frontend/src/components/Dashboard/GuildOverview.tsx`

```tsx
import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { getGuildStreamers, initiateStreamerLink, unlinkStreamer } from '../../services/api';
import { Streamer } from '../../types';
import { LoadingSpinner } from '../common/LoadingSpinner';
import { StreamerCard } from './StreamerCard';
import './GuildOverview.css';

export function GuildOverview() {
  const { guildId } = useParams<{ guildId: string }>();
  const [streamers, setStreamers] = useState<Streamer[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (guildId) {
      loadStreamers();
    }
  }, [guildId]);

  const loadStreamers = async () => {
    if (!guildId) return;
    setLoading(true);
    try {
      const data = await getGuildStreamers(guildId);
      setStreamers(data);
    } finally {
      setLoading(false);
    }
  };

  const handleAddStreamer = async () => {
    if (!guildId) return;
    const { url } = await initiateStreamerLink(guildId);
    window.location.href = url;
  };

  const handleRemoveStreamer = async (streamerId: string) => {
    if (!guildId) return;
    if (confirm('Remove this streamer?')) {
      await unlinkStreamer(guildId, streamerId);
      loadStreamers();
    }
  };

  if (loading) return <LoadingSpinner />;

  return (
    <div className="guild-overview">
      <div className="guild-header">
        <h2>Streamers</h2>
        <button onClick={handleAddStreamer} className="add-button">
          + Add Streamer
        </button>
      </div>

      <div className="streamer-list">
        {streamers.length === 0 ? (
          <p className="empty-state">No streamers added yet. Click "Add Streamer" to get started.</p>
        ) : (
          streamers.map((streamer) => (
            <StreamerCard
              key={streamer.id}
              streamer={streamer}
              onRemove={() => handleRemoveStreamer(streamer.id)}
            />
          ))
        )}
      </div>

      <div className="config-section">
        <h3>Configuration</h3>
        <Link to={`/dashboard/guilds/${guildId}/config`} className="config-button">
          Edit Notification Settings
        </Link>
      </div>
    </div>
  );
}
```

### 9. Streamer Card

**File**: `frontend/src/components/Dashboard/StreamerCard.tsx`

```tsx
import { Streamer } from '../../types';
import './StreamerCard.css';

interface StreamerCardProps {
  streamer: Streamer;
  onRemove: () => void;
}

export function StreamerCard({ streamer, onRemove }: StreamerCardProps) {
  return (
    <div className="streamer-card">
      <img src={streamer.twitch_avatar_url} alt={streamer.twitch_display_name} />
      <div className="streamer-info">
        <h4>{streamer.twitch_display_name}</h4>
        <a
          href={`https://twitch.tv/${streamer.twitch_login}`}
          target="_blank"
          rel="noopener noreferrer"
        >
          @{streamer.twitch_login}
        </a>
      </div>
      <button onClick={onRemove} className="remove-button">
        Remove
      </button>
    </div>
  );
}
```

### 10. Guild Config Editor

**File**: `frontend/src/components/Config/GuildConfigEditor.tsx`

```tsx
import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import {
  getGuildConfig,
  updateGuildConfig,
  getGuildChannels,
  getGuildRoles,
} from '../../services/api';
import { GuildConfig, Channel, Role } from '../../types';
import { LoadingSpinner } from '../common/LoadingSpinner';
import './GuildConfigEditor.css';

export function GuildConfigEditor() {
  const { guildId } = useParams<{ guildId: string }>();
  const [config, setConfig] = useState<GuildConfig | null>(null);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (guildId) {
      Promise.all([
        getGuildConfig(guildId),
        getGuildChannels(guildId),
        getGuildRoles(guildId),
      ])
        .then(([configData, channelsData, rolesData]) => {
          setConfig(configData);
          setChannels(channelsData);
          setRoles(rolesData);
        })
        .finally(() => setLoading(false));
    }
  }, [guildId]);

  const handleSave = async () => {
    if (!guildId || !config) return;
    setSaving(true);
    try {
      await updateGuildConfig(guildId, config);
      alert('Configuration saved!');
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <LoadingSpinner />;
  if (!config) return <ErrorMessage message="Failed to load configuration" />;

  return (
    <div className="config-editor">
      <h2>Notification Configuration</h2>

      <div className="form-group">
        <label>Notification Channel</label>
        <select
          value={config.channel_id}
          onChange={(e) => setConfig({ ...config, channel_id: e.target.value })}
        >
          {channels.map((channel) => (
            <option key={channel.id} value={channel.id}>
              #{channel.name}
            </option>
          ))}
        </select>
      </div>

      <div className="form-group">
        <label>Mention Role (Optional)</label>
        <select
          value={config.mention_role_id || ''}
          onChange={(e) =>
            setConfig({ ...config, mention_role_id: e.target.value || null })
          }
        >
          <option value="">None</option>
          {roles.map((role) => (
            <option key={role.id} value={role.id}>
              @{role.name}
            </option>
          ))}
        </select>
      </div>

      <div className="form-group">
        <label>
          <input
            type="checkbox"
            checked={config.enabled}
            onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
          />
          Enable Notifications
        </label>
      </div>

      <button onClick={handleSave} disabled={saving} className="save-button">
        {saving ? 'Saving...' : 'Save Configuration'}
      </button>
    </div>
  );
}
```

### 11. User Settings

**File**: `frontend/src/components/Settings/UserSettings.tsx`

```tsx
import { useEffect, useState } from 'react';
import { getUserPreferences, updateUserPreference } from '../../services/api';
import { UserPreference } from '../../types';
import { LoadingSpinner } from '../common/LoadingSpinner';
import './UserSettings.css';

export function UserSettings() {
  const [preferences, setPreferences] = useState<UserPreference[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getUserPreferences()
      .then(setPreferences)
      .finally(() => setLoading(false));
  }, []);

  const togglePreference = async (
    guildId: string,
    streamerId: string,
    enabled: boolean
  ) => {
    await updateUserPreference(guildId, streamerId, enabled);
    setPreferences((prev) =>
      prev.map((pref) =>
        pref.guild_id === guildId && pref.streamer_id === streamerId
          ? { ...pref, notifications_enabled: enabled }
          : pref
      )
    );
  };

  if (loading) return <LoadingSpinner />;

  return (
    <div className="user-settings">
      <h2>Notification Preferences</h2>
      <p>Manage which streamers you want to receive notifications for.</p>

      {preferences.length === 0 ? (
        <p className="empty-state">No preferences set.</p>
      ) : (
        <div className="preferences-list">
          {preferences.map((pref) => (
            <div key={`${pref.guild_id}-${pref.streamer_id}`} className="preference-item">
              <span>{/* Streamer name */}</span>
              <label>
                <input
                  type="checkbox"
                  checked={pref.notifications_enabled}
                  onChange={(e) =>
                    togglePreference(pref.guild_id, pref.streamer_id, e.target.checked)
                  }
                />
                Enable notifications
              </label>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
```

### 12. App Router

**File**: `frontend/src/App.tsx`

```tsx
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { LoginPage } from './components/Auth/LoginPage';
import { ProtectedRoute } from './components/Auth/ProtectedRoute';
import { DashboardLayout } from './components/Dashboard/DashboardLayout';
import { GuildSelector } from './components/Dashboard/GuildSelector';
import { GuildOverview } from './components/Dashboard/GuildOverview';
import { GuildConfigEditor } from './components/Config/GuildConfigEditor';
import { UserSettings } from './components/Settings/UserSettings';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<LoginPage />} />
        <Route
          path="/dashboard"
          element={
            <ProtectedRoute>
              <DashboardLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<GuildSelector />} />
          <Route path="guilds/:guildId" element={<GuildOverview />} />
          <Route path="guilds/:guildId/config" element={<GuildConfigEditor />} />
        </Route>
        <Route
          path="/settings"
          element={
            <ProtectedRoute>
              <DashboardLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<UserSettings />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
```

---

## Deployment

### Build for Production

```bash
cd frontend
npm run build
```

### Deploy to S3

```bash
aws s3 sync dist/ s3://streammaxing-frontend --delete
```

### Invalidate CloudFront Cache

```bash
aws cloudfront create-invalidation --distribution-id YOUR_DIST_ID --paths "/*"
```

---

## Testing Checklist

### Manual Testing
- [ ] Login flow works
- [ ] Guild list loads
- [ ] Clicking guild navigates to overview
- [ ] Streamers display correctly
- [ ] Add streamer button redirects to Twitch OAuth
- [ ] Remove streamer works
- [ ] Config editor loads channels and roles
- [ ] Config saves successfully
- [ ] User settings page loads preferences
- [ ] Toggle preference updates database
- [ ] Logout clears session

### UI/UX Testing
- [ ] Responsive design works on mobile
- [ ] Loading states display correctly
- [ ] Error messages are user-friendly
- [ ] Navigation is intuitive
- [ ] Forms validate input

---

## Styling Notes

Use CSS modules or Tailwind CSS for styling. Key design elements:
- Discord-inspired color scheme (dark mode)
- Card-based layout for guilds and streamers
- Clear call-to-action buttons
- Loading spinners for async operations
- Toast notifications for success/error messages

---

## Next Steps

After completing this task:
1. Task 006: Implement edge case handling and cleanup logic
2. Add advanced template editor with live preview
3. Implement template marketplace (community templates)
4. Add analytics dashboard (notification delivery rates)

---

## Notes

- Use environment variable `VITE_API_URL` for API base URL
- Handle CORS properly (credentials: 'include')
- Add error boundaries for better error handling
- Consider adding React Query for data fetching and caching
- Implement optimistic UI updates for better UX
