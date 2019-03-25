# Mindsight Reporting Agent

This application will collect any data from your application that Mindsight is measuring, and send it securely to Mindsight's API for further processing and display on our dashboard.

## Docker Image

New releases are published in a Docker image for easy deployment. To run the agent from the Docker image:

```bash
docker run -d -e MINDSIGHT_CLIENT_ID=<id> -e MINDSIGHT_CLIENT_SECRET=<secret> mindsightco/agent:latest
```

Se [below](#Authentication) for an explanation of the environment variables.

## Download

You can download the latest Mindsight Reporting Agent (just a single Linux binary) from our [release page](https://github.com/MindsightCo/hotpath-agent/releases/latest). You can also build the code in this repository if you have a Go development environment setup, but that may have some unreleased changes that aren't ready for production use. **We recommend you download our latest binary or docker release for production use.**

## Authentication

The following environment variables must be set to properly authenticate with Mindsight's API:

```
MINDSIGHT_CLIENT_ID=<id>
MINDSIGHT_CLIENT_SECRET=<secret>
```

We will provide you the credentials for your account (contact support@mindsight.io).

## Running The Binary

_NOTE:_ You don't need to do this if you've already run the Docker image.

Once the environment variables above have been set and exported, simply run the agent with no arguments:

```
./hotpath-agent
```

The agent will listen on port 8000 by default. Run it with the `-help` flag if you want to customize this, or see other options (mostly useful for development of the agent itself, not needed for normal operation).
