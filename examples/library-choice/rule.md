# Use httpx for HTTP calls

Every HTTP call goes through `httpx`. Do not import `requests`, `urllib` or `aiohttp`. Test code that uses a mock transport is exempt.
