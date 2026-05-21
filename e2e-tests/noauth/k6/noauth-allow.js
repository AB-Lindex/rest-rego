// No-auth soak script for the memory-leak detection run (REQ-012).
// Verified compatible with k6 2.0.0:
//   - http.get(): unchanged
//   - check(): unchanged
//   - __ENV: unchanged
//   - fail(): unchanged
//   - local ES-module import syntax: unchanged

import http from 'k6/http';
import { check, fail } from 'k6';
import { options } from '../../k6-lib/load.js';

export { options };

const PROXY = __ENV.PROXY;
if (!PROXY) {
  fail('PROXY env var is required');
}

const SIZE = __ENV.RESPONSE_SIZE ? `?size=${__ENV.RESPONSE_SIZE}` : '';

export default function () {
  const res = http.get(`${PROXY}/e2e/noauth-allow${SIZE}`);
  check(res, { 'status 200': (r) => r.status === 200 });
}
