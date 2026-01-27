import { useEffect } from 'preact/hooks';
import { Router } from './Router';
import { Toast } from '@components/ui/Toast';
import { NavBar } from '@components/ui';
import { checkAuthStatus, isAuthenticated } from '@store/auth';

export function App() {
  useEffect(() => {
    checkAuthStatus();
  }, []);

  return (
    <div class="app">
      {isAuthenticated.value && <NavBar />}
      <main class="main-content">
        <Router />
      </main>
      <Toast />
    </div>
  );
}
