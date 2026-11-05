// Route parsing and shell selection. The portal sidebar is a staff affordance:
// it is shown only when a staff session is known to exist. Anonymous visitors get
// the public surfaces (parent tracking, queue board, booking) without portal chrome.
export const PARENT_TAB_KEY = 'spr.parentTab';

export type ShellKind = 'portal' | 'parent' | 'kiosk' | 'auth';

export type Route = {
  page: string;
  token: string;
  kiosk: boolean;
  level: string;
  date: string;
};

export type ShellOptions = { staff?: boolean; kiosk?: boolean };

// Hash form: #/track/<token>, #/display?level=..&date=..&kiosk=1, #/book, #/staff
export function parseHash(hash: string): Route {
  const raw = hash.startsWith('#') ? hash.slice(1) : hash;
  const [pathPart = '', queryPart = ''] = raw.split('?');
  const segments = pathPart.split('/').filter(Boolean);
  const page = segments[0] || 'book';
  let token = '';
  if (segments.length > 1) {
    try {
      token = decodeURIComponent(segments.slice(1).join('/'));
    } catch {
      token = '';
    }
  }
  const params = new URLSearchParams(queryPart);
  return {
    page,
    token,
    kiosk: params.get('kiosk') === '1',
    level: params.get('level') ?? '',
    date: params.get('date') ?? '',
  };
}

// The bare site URL is the staff entry: login when anonymous, dashboard when signed in.
export function isBaseEntry(hash: string): boolean {
  const raw = (hash || '').replace(/^#/, '');
  return raw === '' || raw === '/';
}

export function shellFor(page: string, { staff = false, kiosk = false }: ShellOptions = {}): ShellKind {
  // Kiosk mode is an explicit statement about the screen, so it outranks an ambient staff cookie.
  if (kiosk && page === 'display') return 'kiosk';
  if (page === 'display') return staff ? 'portal' : 'kiosk';
  if (page === 'track') return staff ? 'portal' : 'parent';
  if (page === 'staff') return staff ? 'portal' : 'auth';
  return staff ? 'portal' : 'parent';
}

// A tab that arrived through a private tracking link stays on the public surface,
// unless the person is known to be staff.
export function shouldGuard(page: string, parentTab: boolean, staff: boolean): boolean {
  return !staff && parentTab && (page === 'book' || page === 'staff');
}

export function displayHref(level: string, date: string, kiosk = false): string {
  const params = new URLSearchParams();
  if (level) params.set('level', level);
  if (date) params.set('date', date);
  if (kiosk) params.set('kiosk', '1');
  const query = params.toString();
  return `#/display${query ? `?${query}` : ''}`;
}

export function readParentTab(): boolean {
  try {
    return sessionStorage.getItem(PARENT_TAB_KEY) === '1';
  } catch {
    return false;
  }
}

export function markParentTab(): void {
  try {
    sessionStorage.setItem(PARENT_TAB_KEY, '1');
  } catch {
    // private mode / storage disabled: the guard is a convenience, not a security boundary
  }
}

export function clearParentTab(): void {
  try {
    sessionStorage.removeItem(PARENT_TAB_KEY);
  } catch {
    // ignore
  }
}
