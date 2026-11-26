/**
 * The Content-Security-Policy for one request. Scripts run only with this request's nonce, and
 * 'strict-dynamic' lets them load their own chunks. Styles allow inline: the chart library sets
 * style attributes, and injected CSS is far less dangerous than injected script.
 * Development adds 'unsafe-eval' (React uses it for debugging) and the hot-reload websocket.
 */
export function buildCsp(nonce: string, isDev: boolean): string {
  return [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${isDev ? " 'unsafe-eval'" : ""}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data:",
    "font-src 'self'",
    `connect-src 'self'${isDev ? " ws: wss:" : ""}`,
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
  ].join("; ");
}

/** 128 random bits, base64. */
export function newNonce(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return btoa(String.fromCharCode(...bytes));
}
