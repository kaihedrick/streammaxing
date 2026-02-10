import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { LoginPage } from './components/Auth/LoginPage';
import { ProtectedRoute } from './components/Auth/ProtectedRoute';
import { DashboardLayout } from './components/Dashboard/DashboardLayout';
import { GuildSelector } from './components/Dashboard/GuildSelector';
import { GuildOverview } from './components/Dashboard/GuildOverview';
import { GuildConfigEditor } from './components/Config/GuildConfigEditor';
import { UserSettings } from './components/Settings/UserSettings';
import { InvitePage } from './components/Invite/InvitePage';
import './App.css';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<LoginPage />} />
        <Route path="/invite/:code" element={<InvitePage />} />
        <Route
          path="/dashboard"
          element={
            <ProtectedRoute>
              <DashboardLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<GuildSelector />} />
          <Route path="guilds/:guildId" element={<GuildOverview />} />
          <Route path="guilds/:guildId/config" element={<GuildConfigEditor />} />
        </Route>
        <Route
          path="/settings"
          element={
            <ProtectedRoute>
              <DashboardLayout />
            </ProtectedRoute>
          }
        >
          <Route index element={<UserSettings />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
