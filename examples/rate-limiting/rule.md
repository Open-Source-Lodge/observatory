# Rate limit calls to external services

Each call to an external service goes through a rate limiter. A new client for an external service takes a limiter in its constructor. Do not call an external service in a loop without a limiter. Test code with a mock transport is exempt.
