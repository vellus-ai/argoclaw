import { BrowserRouter } from "react-router";
import { AppProviders } from "@/components/providers/app-providers";
import { AppRoutes } from "@/routes";
import { ChangePasswordModal } from "@/components/shared/ChangePasswordModal";

export default function App() {
  return (
    <BrowserRouter>
      <AppProviders>
        <AppRoutes />
        <ChangePasswordModal />
      </AppProviders>
    </BrowserRouter>
  );
}
