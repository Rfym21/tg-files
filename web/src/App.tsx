import { Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./components/AppShell";
import { Login } from "./pages/Login";
import { Files } from "./pages/Files";
import { Links } from "./pages/Links";

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route element={<AppShell />}>
        <Route path="/" element={<Navigate to="/files" replace />} />
        <Route path="/files" element={<Files />} />
        <Route path="/links" element={<Links />} />
      </Route>
      <Route path="*" element={<Navigate to="/files" replace />} />
    </Routes>
  );
}
