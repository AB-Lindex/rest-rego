# Changes since v1.6.4

## Security / Fixes

- **Policy allow handling**
  - An undefined allow (no default allow and no matching rule, or an undefined policy result) is now treated as a deny and returns 403 Forbidden. Previously it returned 500 Internal Server Error.
  - A non-boolean allow (string, number, null) is still rejected with 500 and never forwarded to the backend.
  - Fixed a nil-pointer panic when the policy file was removed at runtime. The request now fails with 500 and a "policy not found" error.

- **Metrics**
  - The method label is now limited to known HTTP methods, and any other value is reported as unknown. This stops clients from creating unbounded metric series with arbitrary methods.

- **Toolchain**
  - Minimum Go version raised from 1.27.0 to 1.27.2, so builds use the patched standard library.

## Tests

- Added table tests for allow handling: true, false, undefined, and non-boolean.
- Added tests for a missing policy file and for method sanitisation.

## Upgrade notes

- Dashboards or alerts that matched 500 for policies with an undefined allow will now see 403.
- Dashboards that filter on unusual method label values will now see unknown.
- Adding default allow := false to a policy keeps the existing behaviour, as the docs already recommend.