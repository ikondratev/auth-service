# auth-service

Internal identity service. It registers a user, signs them in, rotates the session, and revokes it.

Clients reach it only through the API gateway over HTTP. This process speaks gRPC. The first login methods are email and password. Social login and a second factor will attach to the same user later.

Register, login, and refresh return two tokens:

- An access token is a short-lived RS256 JWT. Other services verify it locally with the public key and check the scope they require. event-service already requires `events:write`.
- A refresh token is an opaque value. The service stores only its hash. Refresh revokes the presented token and issues a new pair. Logout revokes that refresh chain, so no further access token can be issued. The current access token stays valid until it expires.

The private key stays in this service. Resource rules stay in the service that owns the data: auth-service does not decide whether a user may create an event.
