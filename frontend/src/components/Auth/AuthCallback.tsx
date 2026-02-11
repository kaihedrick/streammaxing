import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { exchangeDiscordCode } from '../../services/api';
import { LoadingSpinner } from '../common/LoadingSpinner';

/**
 * Handles the Discord OAuth callback on the frontend side.
 *
 * Flow:
 * 1. Discord redirects here with ?code=...&state=...
 * 2. We verify the state against localStorage (CSRF protection)
 * 3. We send the code to the backend to exchange for a session
 * 4. On success, navigate to the dashboard
 *
 * This avoids the cookie-based state check that fails in browsers like Brave
 * which block cookies on cross-site redirects.
 */
export function AuthCallback() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const code = searchParams.get('code');
    const state = searchParams.get('state');
    const oauthError = searchParams.get('error');

    // Discord denied authorization
    if (oauthError) {
      setError('Authorization was denied. Please try again.');
      return;
    }

    if (!code || !state) {
      setError('Missing authorization code or state. Please start login again.');
      return;
    }

    // Verify state against localStorage (CSRF protection)
    const savedState = localStorage.getItem('oauth_state');
    if (!savedState || savedState !== state) {
      localStorage.removeItem('oauth_state');
      setError('Invalid state parameter. Please start login again.');
      return;
    }

    // Clean up the state
    localStorage.removeItem('oauth_state');

    // Exchange code for session
    const redirectUri = `${window.location.origin}/auth/callback`;
    exchangeDiscordCode(code, redirectUri)
      .then(() => {
        navigate('/dashboard', { replace: true });
      })
      .catch((err) => {
        console.error('Auth exchange failed:', err);
        setError('Authentication failed. Please try again.');
      });
  }, [searchParams, navigate]);

  if (error) {
    return (
      <div className="login-page">
        <div className="login-container">
          <h1>Authentication Error</h1>
          <p className="login-subtitle">{error}</p>
          <button onClick={() => navigate('/', { replace: true })} className="discord-button">
            Back to Login
          </button>
        </div>
      </div>
    );
  }

  return <LoadingSpinner />;
}
