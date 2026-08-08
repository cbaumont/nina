# Nina

Nina is an AI pair-programming companion for people who want to *learn* to code, not just have code written for them. Nina is the navigator: she plans a small learning project, explains what to do and why, and reviews what you wrote. You are the driver: you type the code in your own editor. A "typing dial" enforced by the engine (not by asking the model nicely) guarantees Nina can't just do it for you.

The implementation lives in [`python/`](python/), built on the [Claude Agent SDK](https://docs.claude.com/en/api/agent-sdk/overview) so Nina runs on a Claude subscription login (not just an API key), with a local-model option via [Ollama](https://ollama.com). See [`python/README.md`](python/README.md) for install and usage.

Nina snapshots your progress under hidden git refs (`refs/nina/*`); your `git log` and branches stay untouched.

## License

[GPLv3](LICENSE).
