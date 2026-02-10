import { Outlet, Link, useNavigate } from 'react-router-dom';
import { logout } from '../../services/api';
import { useAuth } from '../../hooks/useAuth';

export function DashboardLayout() {
  const navigate = useNavigate();
  const { user } = useAuth();

  const handleLogout = async () => {
    try {
      await logout();
    } catch {
      // Ignore logout errors
    }
    navigate('/');
  };

  return (
    <div className="dashboard-layout">
      <header className="dashboard-header">
        <div className="header-left">
          <Link to="/dashboard" className="header-logo">
            StreamMaxing
          </Link>
        </div>
        <nav className="header-nav">
          <Link to="/dashboard" className="nav-link">Servers</Link>
          <Link to="/settings" className="nav-link">Settings</Link>
          <div className="header-user">
            {user && (
              <span className="user-name">{user.username}</span>
            )}
            <button onClick={handleLogout} className="btn btn-secondary btn-sm">
              Logout
            </button>
          </div>
        </nav>
      </header>
      <main className="dashboard-content">
        <Outlet />
      </main>
    </div>
  );
}
