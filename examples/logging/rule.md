# No print statements

Use the logger, not `print`. A module gets its logger with `logging.getLogger(__name__)`. Command line entry points under `cli/` may print to the user.
