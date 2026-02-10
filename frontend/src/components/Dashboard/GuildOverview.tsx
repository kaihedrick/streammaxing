import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  getGuildStreamers,
  initiateStreamerLink,
  unlinkStreamer,
  getBotInstallURL,
  getUserGuilds,
  createInviteLink,
  getInviteLinks,
  deleteInviteLink,
} from '../../services/api';
import type { Streamer, Guild, InviteLink } from '../../types';
import { LoadingSpinner } from '../common/LoadingSpinner';
import { StreamerCard } from './StreamerCard';
import { StreamerMessageEditor } from './StreamerMessageEditor';
import { useAuth } from '../../hooks/useAuth';

export function GuildOverview() {
  const { guildId } = useParams<{ guildId: string }>();
  const { user } = useAuth();
  const [guild, setGuild] = useState<Guild | null>(null);
  const [streamers, setStreamers] = useState<Streamer[]>([]);
  const [invites, setInvites] = useState<InviteLink[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [installingBot, setInstallingBot] = useState(false);
  const [editingStreamerId, setEditingStreamerId] = useState<string | null>(null);

  const isAdmin = guild?.is_admin ?? false;

  useEffect(() => {
    if (guildId) {
      loadData();
    }
  }, [guildId]);

  const loadData = async () => {
    if (!guildId) return;
    setLoading(true);
    try {
      const [streamersData, guildsData] = await Promise.all([
        getGuildStreamers(guildId),
        getUserGuilds(),
      ]);
      setStreamers(streamersData);
      const currentGuild = guildsData.find((g) => g.guild_id === guildId);
      setGuild(currentGuild || null);

      // Load invites if admin
      if (currentGuild?.is_admin) {
        try {
          const invitesData = await getInviteLinks(guildId);
          setInvites(invitesData);
        } catch {
          // Non-fatal
        }
      }
    } catch {
      // Handle error silently for now
    } finally {
      setLoading(false);
    }
  };

  const handleAddStreamer = async () => {
    if (!guildId || adding) return;
    setAdding(true);
    try {
      const { url } = await initiateStreamerLink(guildId);
      window.location.href = url;
    } catch {
      setAdding(false);
      alert('Failed to initiate streamer linking. Please try again.');
    }
  };

  const handleRemoveStreamer = async (streamerId: string) => {
    if (!guildId) return;
    if (!confirm('Are you sure you want to remove this streamer?')) return;
    try {
      await unlinkStreamer(guildId, streamerId);
      loadData();
    } catch {
      alert('Failed to remove streamer. Please try again.');
    }
  };

  const handleInstallBot = async () => {
    if (!guildId || installingBot) return;
    setInstallingBot(true);
    try {
      const { url } = await getBotInstallURL(guildId);
      window.open(url, '_blank');
    } catch {
      alert('Failed to get bot install URL. Please try again.');
    } finally {
      setInstallingBot(false);
    }
  };

  const handleCreateInvite = async () => {
    if (!guildId) return;
    try {
      const link = await createInviteLink(guildId);
      setInvites((prev) => [link, ...prev]);
    } catch {
      alert('Failed to create invite link.');
    }
  };

  const handleDeleteInvite = async (inviteId: string) => {
    if (!guildId) return;
    try {
      await deleteInviteLink(guildId, inviteId);
      setInvites((prev) => prev.filter((i) => i.id !== inviteId));
    } catch {
      alert('Failed to delete invite link.');
    }
  };

  const copyInviteLink = (code: string) => {
    const url = `${window.location.origin}/invite/${code}`;
    navigator.clipboard.writeText(url);
  };

  if (loading) return <LoadingSpinner />;

  return (
    <div className="guild-overview">
      <div className="page-header">
        <div>
          <Link to="/dashboard" className="back-link">← Back to Servers</Link>
          <h2>
            Streamers
            {guild && (
              <span className={`guild-badge ${isAdmin ? 'admin' : 'member'}`} style={{ marginLeft: 10, verticalAlign: 'middle' }}>
                {isAdmin ? 'Admin' : 'Member'}
              </span>
            )}
          </h2>
        </div>
        {isAdmin && (
          <button onClick={handleAddStreamer} className="btn btn-primary" disabled={adding}>
            {adding ? 'Connecting...' : '+ Add Streamer'}
          </button>
        )}
      </div>

      {/* Bot Install - admin only */}
      {isAdmin && (
        <div className="bot-install-section">
          <div className="bot-install-card">
            <div className="bot-install-info">
              <h3>Bot Installation</h3>
              <p>The bot must be added to your Discord server to send notifications and fetch channels/roles.</p>
            </div>
            <button onClick={handleInstallBot} className="btn btn-secondary" disabled={installingBot}>
              {installingBot ? 'Loading...' : 'Add Bot to Server'}
            </button>
          </div>
        </div>
      )}

      {/* Streamer list */}
      <div className="streamer-list">
        {streamers.length === 0 ? (
          <div className="empty-state">
            {isAdmin ? (
              <p>No streamers added yet. Click "+ Add Streamer" to link a Twitch streamer, or share an invite link.</p>
            ) : (
              <p>No streamers have been added to this server yet. Ask the server admin to add streamers or use an invite link to link your Twitch.</p>
            )}
          </div>
        ) : (
          streamers.map((streamer) => {
            const isOwner = streamer.added_by === user?.user_id;
            return (
              <div key={streamer.id}>
                <StreamerCard
                  streamer={streamer}
                  onRemove={isAdmin ? () => handleRemoveStreamer(streamer.id) : undefined}
                  onEditMessage={(isAdmin || isOwner) ? () => setEditingStreamerId(
                    editingStreamerId === streamer.id ? null : streamer.id
                  ) : undefined}
                  isOwner={isOwner}
                />
                {editingStreamerId === streamer.id && guildId && (
                  <StreamerMessageEditor
                    guildId={guildId}
                    streamerId={streamer.id}
                    streamerName={streamer.twitch_display_name}
                    initialContent={streamer.custom_content || ''}
                  />
                )}
              </div>
            );
          })
        )}
      </div>

      {/* Config - admin only */}
      {isAdmin && (
        <div className="config-section">
          <h3>Notification Settings</h3>
          <p className="section-subtitle">Configure how notifications are sent in this server</p>
          <Link to={`/dashboard/guilds/${guildId}/config`} className="btn btn-secondary">
            Edit Configuration
          </Link>
        </div>
      )}

      {/* Invite Links - admin only */}
      {isAdmin && (
        <div className="invite-management">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <h3>Invite Links</h3>
            <button onClick={handleCreateInvite} className="btn btn-secondary btn-sm">
              Generate Link
            </button>
          </div>
          <p className="section-subtitle" style={{ margin: '4px 0 0', fontSize: '0.85rem', color: '#b9bbbe' }}>
            Share these links with streamers so they can join and link their Twitch accounts.
          </p>
          {invites.length > 0 && (
            <div className="invite-list">
              {invites.map((invite) => (
                <div key={invite.id} className="invite-item">
                  <span
                    className="invite-item-code"
                    onClick={() => copyInviteLink(invite.code)}
                    title="Click to copy full link"
                  >
                    {window.location.origin}/invite/{invite.code}
                  </span>
                  <span className="invite-item-meta">
                    {invite.use_count} uses
                    {invite.max_uses > 0 && ` / ${invite.max_uses} max`}
                  </span>
                  <button
                    onClick={() => handleDeleteInvite(invite.id)}
                    className="btn btn-danger btn-sm"
                  >
                    Delete
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
