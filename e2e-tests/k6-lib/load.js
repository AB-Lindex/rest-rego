// Shared k6 load preset for soak / memory-leak runs.
// Verified compatible with k6 2.0.0:
//   - ramping-vus executor: unchanged
//   - stages array shape: unchanged
//   - thresholds schema: unchanged
//   - http_req_failed metric name: unchanged

export const options = {
  scenarios: {
    sustained: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '2m', target: 100 },
        { duration: '5m', target: 100 },
      ],
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
  },
};
