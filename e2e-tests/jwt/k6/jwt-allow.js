// JWT soak script for the memory-leak investigation (jwt-policy-memory-leak).
// Each VU fetches a fresh bearer token from the helper server once during init,
// then sends sustained requests to the rest-rego proxy.
//
// Verified compatible with k6 2.0.0:
//   - http.get(), http.post(): unchanged
//   - check(), fail(): unchanged
//   - __ENV, __VU: unchanged

import http from 'k6/http';
import { check, fail } from 'k6';
import { options } from '../../k6-lib/load.js';

export { options };

const PROXY = __ENV.PROXY;
if (!PROXY) {
  fail('PROXY env var is required');
}

const TOKEN_URL = __ENV.TOKEN_URL;
if (!TOKEN_URL) {
  fail('TOKEN_URL env var is required (e.g. http://127.0.0.1:18305/token)');
}

const SIZE = __ENV.RESPONSE_SIZE ? `?size=${__ENV.RESPONSE_SIZE}` : '';

// Fetch a token once per VU during init phase.
// This exercises the JWT validation path on every request without the overhead
// of token generation, isolating any memory growth to the auth+policy path.
const tokenRes = http.get(TOKEN_URL);
if (tokenRes.status !== 200) {
  fail(`failed to fetch token: status ${tokenRes.status}`);
}
const bearerToken = tokenRes.body;

export default function () {
  const res = http.get(`${PROXY}/e2e/jwt-allow${SIZE}`, {
    headers: { Authorization: `Bearer ${bearerToken}` },
  });
  check(res, { 'status 200': (r) => r.status === 200 });
}
