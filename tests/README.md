# Tests

## Manual / Developer testing

This is our `.http`-files and we use the VS-Code extension called [REST Client](https://marketplace.visualstudio.com/items?itemName=humao.rest-client) along with a `.env`-file in the project root that looks something like this:
```env
TENANT=azure-tenant-id
CLIENT_ID=application-id-to-test-with
CLIENT_SECRET=the-password-of-that-app
```

## Programatical / Performance-testing

This are the `.k6`-files using the [Grafana k6](https://k6.io/) project.

Test-setup is using Rancher Desktop and deployed using the [Kubernetes-examples](../examples/kubernetes) in this repo.
(and the demo.local is an alias to localhost)