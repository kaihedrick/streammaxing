import { useState } from 'react';
import { updateStreamerMessage } from '../../services/api';

interface StreamerMessageEditorProps {
  guildId: string;
  streamerId: string;
  streamerName: string;
  initialContent: string;
}

const PLACEHOLDERS = [
  { key: '{streamer_display_name}', desc: 'Streamer name' },
  { key: '{stream_title}', desc: 'Stream title' },
  { key: '{game_name}', desc: 'Game being played' },
  { key: '{viewer_count}', desc: 'Current viewers' },
  { key: '{mention_role}', desc: 'Mention role (if set)' },
];

export function StreamerMessageEditor({ guildId, streamerId, streamerName, initialContent }: StreamerMessageEditorProps) {
  const [content, setContent] = useState(initialContent);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    setSaved(false);
    try {
      await updateStreamerMessage(guildId, streamerId, content);
      setSaved(true);
      setTimeout(() => setSaved(false), 3000);
    } catch {
      alert('Failed to save message. Please try again.');
    } finally {
      setSaving(false);
    }
  };

  const handleClear = () => {
    setContent('');
  };

  // Live preview with dummy data
  const preview = content
    ? content
        .replace(/\{streamer_display_name\}/g, streamerName)
        .replace(/\{stream_title\}/g, 'Playing some games!')
        .replace(/\{game_name\}/g, 'Just Chatting')
        .replace(/\{viewer_count\}/g, '142')
        .replace(/\{mention_role\}/g, '@everyone')
    : null;

  return (
    <div className="message-editor">
      <h3>Custom Notification Text for {streamerName}</h3>
      <p className="message-editor-help">
        Customize the text message sent when this streamer goes live. Leave empty to use the server default.
        The embed (rich preview) is always controlled by the server admin.
      </p>

      <textarea
        value={content}
        onChange={(e) => setContent(e.target.value)}
        placeholder="e.g. Hey everyone! {streamer_display_name} is now live playing {game_name}! {mention_role}"
      />

      <p className="message-editor-help">
        <strong>Available placeholders:</strong>{' '}
        {PLACEHOLDERS.map((p, i) => (
          <span key={p.key}>
            <code
              style={{ cursor: 'pointer', color: '#7289da' }}
              onClick={() => setContent((prev) => prev + p.key)}
              title={`Click to insert - ${p.desc}`}
            >
              {p.key}
            </code>
            {i < PLACEHOLDERS.length - 1 ? ', ' : ''}
          </span>
        ))}
      </p>

      {preview && (
        <div style={{ background: '#36393f', borderRadius: 6, padding: '10px 14px', marginBottom: 12 }}>
          <p className="message-editor-help" style={{ margin: '0 0 4px', color: '#72767d' }}>Preview:</p>
          <p style={{ margin: 0, color: '#dcddde', fontSize: '0.9rem' }}>{preview}</p>
        </div>
      )}

      <div className="message-editor-actions">
        <button onClick={handleSave} disabled={saving} className="btn btn-primary btn-sm">
          {saving ? 'Saving...' : saved ? 'Saved!' : 'Save Message'}
        </button>
        <button onClick={handleClear} className="btn btn-secondary btn-sm">
          Clear (Use Default)
        </button>
      </div>
    </div>
  );
}
