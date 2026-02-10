import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { getUserGuilds } from '../../services/api';
import type { Guild } from '../../types';
import { LoadingSpinner } from '../common/LoadingSpinner';
import { ErrorMessage } from '../common/ErrorMessage';

export function GuildSelector() {
  const [guilds, setGuilds] = useState<Guild[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const navigate = useNavigate();

  const loadGuilds = () => {
    setLoading(true);
    setError(null);
    getUserGuilds()
      .then(setGuilds)
      .catch(() => setError('Failed to load servers'))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    loadGuilds();
  }, []);

  if (loading) return <LoadingSpinner />;
  if (error) return <ErrorMessage message={error} onRetry={loadGuilds} />;

  return (
    <div className="guild-selector">
      <h2>Your Servers</h2>
      <p className="section-subtitle">Select a server to manage stream notifications</p>
      {guilds.length === 0 ? (
        <div className="empty-state">
          <p>No servers found. Make sure you're a member of a server with the StreamMaxing bot, or ask an admin for an invite link.</p>
        </div>
      ) : (
        <div className="guild-grid">
          {guilds.map((guild) => (
            <div
              key={guild.guild_id}
              className="guild-card"
              onClick={() => navigate(`/dashboard/guilds/${guild.guild_id}`)}
            >
              <div className="guild-icon">
                {guild.icon ? (
                  <img
                    src={`https://cdn.discordapp.com/icons/${guild.guild_id}/${guild.icon}.png?size=128`}
                    alt={guild.name}
                  />
                ) : (
                  <div className="guild-icon-placeholder">
                    {guild.name.charAt(0).toUpperCase()}
                  </div>
                )}
              </div>
              <h3 className="guild-name">{guild.name}</h3>
              <span className={`guild-badge ${guild.is_admin ? 'admin' : 'member'}`}>
                {guild.is_admin ? 'Admin' : 'Member'}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
