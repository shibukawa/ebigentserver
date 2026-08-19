---
id: requirement:token-based-identity
type: requirement
title: Token Based Identity
---
Framework identifies players by signed tokens only and never handles credentials or an account store.

```yaml
framework_never_holds: passwords, oauth exchanges, account records, session cookies
framework_holds: a verification key and the claims of data:session-ticket
issuer: concept:control-plane, any implementation
encoding: decision:jwt-session-ticket
serves: requirement:control-plane-decoupling
consequence: swapping the identity provider changes no framework code
```
