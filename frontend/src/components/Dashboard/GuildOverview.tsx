import { useEffect, useState, useCallback } from 'react';
import { useParams, Link, useSearchParams } from 'react-router-dom';
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
  const [searchParams, setSearchParams] = useSearchParams();
  const { user } = useAuth();
  const [guild, setGuild] = useState<Guild | null>(null);
  const [streamers, setStreamers] = useState<Streamer[]>([]);
  const [invites, setInvites] = useState<InviteLink[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [installingBot, setInstallingBot] = useState(false);
  const [editingStreamerId, setEditingStreamerId] = useState<string | null>(null);
  const [notice, setNotice] = useState<{ message: string; type: 'info' | 'error' } | null>(null);

  const isAdmin = guild?.is_admin ?? false;

  // Check for notices/errors from redirects
  useEffect(() => {
    const noticeParam = searchParams.get('notice');
    const errorParam = searchParams.get('error');
    let handled = false;

    if (noticeParam === 'already_linked') {
      const streamerName = searchParams.get('streamer') || 'This streamer';
      setNotice({ message: `${streamerName} is already linked to this server.`, type: 'info' });
      handled = true;
    } else if (errorParam === 'twitch_auth_denied') {
      setNotice({ message: 'Twitch authorization failed or was denied. Please try again.', type: 'error' });
      handled = true;
    }

    if (handled) {
      // Clean all query params from the URL
      setSearchParams({}, { replace: true });
      // Auto-dismiss after 6 seconds
      setTimeout(() => setNotice(null), 6000);
    }
  }, [searchParams, setSearchParams]);

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

  const [copiedId, setCopiedId] = useState<string | null>(null);

  const copyInviteLink = useCallback((invite: InviteLink) => {
    const url = `${window.location.origin}/invite/${invite.code}`;
    navigator.clipboard.writeText(url).then(() => {
      setCopiedId(invite.id);
      setTimeout(() => setCopiedId(null), 2000);
    });
  }, []);

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

      {/* Notice / error banner */}
      {notice && (
        <div className={`notice-banner ${notice.type === 'error' ? 'notice-error' : ''}`}>
          <span>{notice.message}</span>
          <button className="notice-dismiss" onClick={() => setNotice(null)}>&times;</button>
        </div>
      )}

      {/* Bot Install hint - only show for admins when no streamers are set up yet.
          If streamers exist, the bot is clearly working; no need to show this. */}
      {isAdmin && streamers.length === 0 && (
        <div className="bot-install-section">
          <div className="bot-install-card">
            <div className="bot-install-info">
              <h3>Getting Started</h3>
              <p>If you haven't already, add the StreamMaxing bot to your server, then use "+ Add Streamer" above to link a Twitch channel.</p>
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
                  <code className="invite-item-code">{window.location.origin}/invite/{invite.code}</code>
                  <button
                    className="btn-icon copy-btn"
                    onClick={() => copyInviteLink(invite)}
                    title="Copy invite link"
                  >
                    {copiedId === invite.id ? (
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <polyline points="20 6 9 17 4 12" />
                      </svg>
                    ) : (
                      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                        <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                      </svg>
                    )}
                    <span className="copy-label">{copiedId === invite.id ? 'Copied!' : 'Copy'}</span>
                  </button>
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
