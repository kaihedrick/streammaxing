import { useEffect, useState } from 'react';
import { getUserPreferences, updateUserPreference } from '../../services/api';
import type { UserPreference } from '../../types';
import { LoadingSpinner } from '../common/LoadingSpinner';

export function UserSettings() {
  const [preferences, setPreferences] = useState<UserPreference[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getUserPreferences()
      .then(setPreferences)
      .catch(() => {
        // Handle error silently
      })
      .finally(() => setLoading(false));
  }, []);

  const togglePreference = async (
    guildId: string,
    streamerId: string,
    enabled: boolean
  ) => {
    try {
      await updateUserPreference(guildId, streamerId, enabled);
      setPreferences((prev) =>
        prev.map((pref) =>
          pref.guild_id === guildId && pref.streamer_id === streamerId
            ? { ...pref, notifications_enabled: enabled }
            : pref
        )
      );
    } catch {
      alert('Failed to update preference. Please try again.');
    }
  };

  if (loading) return <LoadingSpinner />;

  return (
    <div className="user-settings">
      <h2>Notification Preferences</h2>
      <p className="section-subtitle">
        Manage which streamers you want to receive notifications for in each server.
      </p>

      {preferences.length === 0 ? (
        <div className="empty-state">
          <p>No notification preferences set yet. Preferences will appear here once you customize them.</p>
        </div>
      ) : (
        <div className="preferences-list">
          {preferences.map((pref) => (
            <div
              key={`${pref.guild_id}-${pref.streamer_id}`}
              className="preference-item"
            >
              <div className="preference-info">
                <span className="preference-guild">Server: {pref.guild_id}</span>
                <span className="preference-streamer">Streamer: {pref.streamer_id}</span>
              </div>
              <label className="toggle-label">
                <input
                  type="checkbox"
                  checked={pref.notifications_enabled}
                  onChange={(e) =>
                    togglePreference(
                      pref.guild_id,
                      pref.streamer_id,
                      e.target.checked
                    )
                  }
                />
                <span>Notifications {pref.notifications_enabled ? 'On' : 'Off'}</span>
              </label>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
