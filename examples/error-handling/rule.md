# Do not ignore errors

Each error that a call returns is handled or returned to the caller. Do not assign an error to `_`. Do not catch an exception and do nothing in the handler. A comment in the handler that says why the error is safe to ignore makes the handler exempt.
