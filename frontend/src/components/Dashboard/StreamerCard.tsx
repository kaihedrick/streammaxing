import type { Streamer } from '../../types';

interface StreamerCardProps {
  streamer: Streamer;
  onRemove?: () => void;
  onEditMessage?: () => void;
  isOwner?: boolean;
}

export function StreamerCard({ streamer, onRemove, onEditMessage, isOwner }: StreamerCardProps) {
  return (
    <div className="streamer-card">
      <img
        src={streamer.twitch_avatar_url || 'https://static-cdn.jtvnw.net/user-default-pictures-uv/75305d54-c7cc-40d1-bb9c-91fbe85943c7-profile_image-70x70.png'}
        alt={streamer.twitch_display_name}
        className="streamer-avatar"
      />
      <div className="streamer-info">
        <h4>
          {streamer.twitch_display_name}
          {isOwner && <span className="guild-badge member" style={{ marginLeft: 8, fontSize: '0.65rem' }}>You</span>}
        </h4>
        <a
          href={`https://twitch.tv/${streamer.twitch_login}`}
          target="_blank"
          rel="noopener noreferrer"
          className="streamer-link"
        >
          twitch.tv/{streamer.twitch_login}
        </a>
      </div>
      <div style={{ display: 'flex', gap: 6 }}>
        {onEditMessage && (
          <button onClick={onEditMessage} className="btn btn-secondary btn-sm">
            Edit Message
          </button>
        )}
        {onRemove && (
          <button onClick={onRemove} className="btn btn-danger btn-sm">
            Remove
          </button>
        )}
      </div>
    </div>
  );
}
