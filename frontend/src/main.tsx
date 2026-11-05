import './styles.css';
import { createRoot } from 'react-dom/client';
import { App } from './App';
import { registerServiceWorker } from './sw-register';
createRoot(document.getElementById('root')!).render(<App />);

// Enable the offline app shell. Production-only, after load, never throws.
registerServiceWorker();
