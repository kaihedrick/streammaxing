import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  getGuildConfig,
  updateGuildConfig,
  getGuildChannels,
  getGuildRoles,
  getBotInstallURL,
} from '../../services/api';
import type { GuildConfig, Channel, Role } from '../../types';
import { LoadingSpinner } from '../common/LoadingSpinner';

export function GuildConfigEditor() {
  const { guildId } = useParams<{ guildId: string }>();
  const [config, setConfig] = useState<GuildConfig | null>(null);
  const [channels, setChannels] = useState<Channel[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (guildId) {
      loadData();
    }
  }, [guildId]);

  const loadData = async () => {
    if (!guildId) return;
    setLoading(true);
    setError(null);
    try {
      const [configData, channelsData, rolesData] = await Promise.all([
        getGuildConfig(guildId),
        getGuildChannels(guildId),
        getGuildRoles(guildId),
      ]);
      setConfig(configData);
      setChannels(channelsData);
      setRoles(rolesData);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      // If channels/roles fail, the bot likely isn't installed
      setError(message.includes('403') || message.includes('50001')
        ? 'bot_not_installed'
        : 'bot_not_installed'); // Default to this since it's the most common cause
    } finally {
      setLoading(false);
    }
  };

  const handleInstallBot = async () => {
    if (!guildId) return;
    try {
      const { url } = await getBotInstallURL(guildId);
      window.open(url, '_blank');
    } catch {
      alert('Failed to get bot install URL.');
    }
  };

  const handleSave = async () => {
    if (!guildId || !config) return;
    setSaving(true);
    setSaved(false);
    try {
      await updateGuildConfig(guildId, config);
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch {
      alert('Failed to save configuration. Please try again.');
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <LoadingSpinner />;
  if (error) {
    return (
      <div className="config-editor">
        <div className="page-header">
          <div>
            <Link to={`/dashboard/guilds/${guildId}`} className="back-link">← Back to Server</Link>
            <h2>Notification Configuration</h2>
          </div>
        </div>
        <div className="bot-install-card">
          <div className="bot-install-info">
            <h3>Bot Not Installed</h3>
            <p>The StreamMaxing bot must be added to this Discord server before you can configure notifications. Click below to install it, then come back and retry.</p>
          </div>
          <div className="bot-install-actions">
            <button onClick={handleInstallBot} className="btn btn-primary">
              Add Bot to Server
            </button>
            <button onClick={loadData} className="btn btn-secondary">
              Retry
            </button>
          </div>
        </div>
      </div>
    );
  }
  if (!config) return (
    <div className="config-editor">
      <div className="page-header">
        <div>
          <Link to={`/dashboard/guilds/${guildId}`} className="back-link">← Back to Server</Link>
          <h2>Notification Configuration</h2>
        </div>
      </div>
      <p>Configuration not found.</p>
    </div>
  );

  return (
    <div className="config-editor">
      <div className="page-header">
        <div>
          <Link to={`/dashboard/guilds/${guildId}`} className="back-link">← Back to Server</Link>
          <h2>Notification Configuration</h2>
        </div>
      </div>

      <div className="config-form">
        <div className="form-group">
          <label htmlFor="channel">Notification Channel</label>
          <select
            id="channel"
            value={config.channel_id}
            onChange={(e) => setConfig({ ...config, channel_id: e.target.value })}
          >
            <option value="">Select a channel...</option>
            {channels.map((channel) => (
              <option key={channel.id} value={channel.id}>
                #{channel.name}
              </option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label htmlFor="role">Mention Role (Optional)</label>
          <select
            id="role"
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

        <div className="form-group form-group-checkbox">
          <label>
            <input
              type="checkbox"
              checked={config.enabled}
              onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
            />
            <span>Enable Notifications</span>
          </label>
          <p className="form-help">When disabled, no notifications will be sent to this server.</p>
        </div>

        <div className="form-actions">
          <button onClick={handleSave} disabled={saving} className="btn btn-primary">
            {saving ? 'Saving...' : saved ? 'Saved!' : 'Save Configuration'}
          </button>
        </div>
      </div>
    </div>
  );
}
