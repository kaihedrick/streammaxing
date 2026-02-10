import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { getInviteInfo, acceptInvite, loginWithDiscord, initiateStreamerLink } from '../../services/api';
import { useAuth } from '../../hooks/useAuth';
import type { InviteInfo } from '../../types';
import { LoadingSpinner } from '../common/LoadingSpinner';

export function InvitePage() {
  const { code } = useParams<{ code: string }>();
  const navigate = useNavigate();
  const { isAuthenticated, isLoading: authLoading } = useAuth();
  const [invite, setInvite] = useState<InviteInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [accepting, setAccepting] = useState(false);
  const [accepted, setAccepted] = useState(false);
  const [linking, setLinking] = useState(false);

  useEffect(() => {
    if (code) {
      loadInvite();
    }
  }, [code]);

  const loadInvite = async () => {
    if (!code) return;
    setLoading(true);
    try {
      const info = await getInviteInfo(code);
      setInvite(info);
    } catch {
      setError('This invite link is invalid or has expired.');
    } finally {
      setLoading(false);
    }
  };

  const handleAccept = async () => {
    if (!code) return;
    setAccepting(true);
    try {
      await acceptInvite(code);
      setAccepted(true);
    } catch {
      setError('Failed to accept invite. Please try again.');
    } finally {
      setAccepting(false);
    }
  };

  const handleLinkTwitch = async () => {
    if (!invite) return;
    setLinking(true);
    try {
      const { url } = await initiateStreamerLink(invite.guild_id);
      window.location.href = url;
    } catch {
      setLinking(false);
      alert('Failed to start Twitch linking. Please try again.');
    }
  };

  const handleLoginAndAccept = () => {
    // Store the invite code so we can accept after login
    sessionStorage.setItem('pending_invite', code || '');
    loginWithDiscord();
  };

  // Auto-accept invite after login redirect
  useEffect(() => {
    if (isAuthenticated && !accepted && !accepting) {
      const pendingInvite = sessionStorage.getItem('pending_invite');
      if (pendingInvite === code) {
        sessionStorage.removeItem('pending_invite');
        handleAccept();
      }
    }
  }, [isAuthenticated]);

  if (loading || authLoading) return <LoadingSpinner />;

  if (error) {
    return (
      <div className="invite-page">
        <div className="invite-card">
          <div className="invite-error">
            <h2>Invalid Invite</h2>
            <p>{error}</p>
          </div>
        </div>
      </div>
    );
  }

  if (!invite) return null;

  return (
    <div className="invite-page">
      <div className="invite-card">
        <div className="invite-guild-info">
          <div className="invite-guild-icon">
            {invite.guild_icon ? (
              <img
                src={`https://cdn.discordapp.com/icons/${invite.guild_id}/${invite.guild_icon}.png?size=128`}
                alt={invite.guild_name}
              />
            ) : (
              <div className="guild-icon-placeholder large">
                {invite.guild_name.charAt(0).toUpperCase()}
              </div>
            )}
          </div>
          <h2>{invite.guild_name}</h2>
          <p className="invite-subtitle">You've been invited to join this server's stream notification system</p>
        </div>

        {!isAuthenticated ? (
          <div className="invite-actions">
            <p className="invite-step">Step 1: Log in with Discord</p>
            <button onClick={handleLoginAndAccept} className="btn btn-discord">
              Login with Discord
            </button>
          </div>
        ) : accepted ? (
          <div className="invite-actions">
            <div className="invite-success">
              <p>You've joined {invite.guild_name}!</p>
            </div>
            <p className="invite-step">Step 2: Link your Twitch account</p>
            <button onClick={handleLinkTwitch} className="btn btn-twitch" disabled={linking}>
              {linking ? 'Connecting...' : 'Link Your Twitch'}
            </button>
            <button onClick={() => navigate(`/dashboard/guilds/${invite.guild_id}`)} className="btn btn-secondary">
              Go to Dashboard
            </button>
          </div>
        ) : (
          <div className="invite-actions">
            <button onClick={handleAccept} className="btn btn-primary" disabled={accepting}>
              {accepting ? 'Joining...' : 'Accept Invite'}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
