// Route parsing and shell selection for the public tracking link and the queue board.
// Pure functions (plus guarded sessionStorage access) so they can be unit tested.
export const PARENT_TAB_KEY = 'spr.parentTab';

export type ShellKind = 'portal' | 'parent' | 'kiosk';

export type Route = {
  page: string;
  token: string;
  kiosk: boolean;
  level: string;
  date: string;
};

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

export function shellFor(page: string): ShellKind {
  if (page === 'track') return 'parent';
  if (page === 'display') return 'kiosk';
  return 'portal';
}

// A tab that arrived through a private tracking link stays on the parent surface.
export function guardsPortalRoute(page: string, parentTab: boolean): boolean {
  return parentTab && (page === 'book' || page === 'staff');
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
