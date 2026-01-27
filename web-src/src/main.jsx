import { render } from 'preact';
import { App } from './App';

import '@styles/tokens.css';
import '@styles/base.css';
import '@styles/control-panel.css';
import '@styles/modals.css';
import '@styles/filters.css';
import '@styles/components.css';
import '@styles/tooltips.css';

import '@styles/dashboard.css';
import '@styles/auth.css';

render(<App />, document.getElementById('app'));
