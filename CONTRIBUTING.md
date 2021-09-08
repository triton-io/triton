# Contributing to Triton

Welcome to Triton! Triton consists several repositories under the organization.
We encourage you to help out by reporting issues, improving documentation, fixing bugs, or adding new features.
Please also take a look at our code of conduct, which details how contributors are expected to conduct themselves as part of the Triton community.

## Reporting issues

To be honest, we regard every user of Triton as a very kind contributor.
After experiencing Triton, you may have some feedback for the project.
Then feel free to open an issue.

There are lot of cases when you could open an issue:

- bug report
- feature request
- performance issues
- feature proposal
- feature design
- help wanted
- doc incomplete
- test improvement
- any questions on project
- and so on

Also we must remind that when filing a new issue, please remember to remove the sensitive data from your post.
Sensitive data could be password, secret key, network locations, private business data and so on.

## Code and doc contribution

Every action to make Triton better is encouraged.
On GitHub, every improvement for Triton could be via a PR (short for pull request).

- If you find a typo, try to fix it!
- If you find a bug, try to fix it!
- If you find some redundant codes, try to remove them!
- If you find some test cases missing, try to add them!
- If you could enhance a feature, please DO NOT hesitate!
- If you find code implicit, try to add comments to make it clear!
- If you find code ugly, try to refactor that!
- If you can help to improve documents, it could not be better!
- If you find document incorrect, just do it and fix that!
- ...

### Workspace Preparation

To put forward a PR, we assume you have registered a GitHub ID.
Then you could finish the preparation in the following steps:

1. **Fork** Fork the repository you wish to work on. You just need to click the button Fork in right-left of project repository main page. Then you will end up with your repository in your GitHub username.
2. **Clone** your own repository to develop locally. Use `git clone https://github.com/<your-username>/<project>.git` to clone repository to your local machine. Then you can create new branches to finish the change you wish to make.
3. **Set remote** upstream to be `https://github.com/Triton/<project>.git` using the following two commands:

```bash
git remote add upstream https://github.com/Triton/<project>.git
git remote set-url --push upstream no-pushing
```

Adding this, we can easily synchronize local branches with upstream branches.

4. **Create a branch** to add a new feature or fix issues

Update local working directory:

```bash
cd <project>
git fetch upstream
git checkout master
git rebase upstream/master
```

Create a new branch:

```bash
git checkout -b <new-branch>
```

Make any change on the new-branch then build and test your codes.

### PR Description

PR is the only way to make change to Triton project files.
To help reviewers better get your purpose, PR description could not be too detailed.
We encourage contributors to follow the [PR template](./.github/PULL_REQUEST_TEMPLATE.md) to finish the pull request.

### Developing Environment

As a contributor, if you want to make any contribution to Triton project, we should reach an agreement on the version of tools used in the development environment.
Here are some dependents with specific version:

- Golang : v1.13+ (1.14 is best)
- Kubernetes: v1.12+

### Developing guide

There's a `Makefile` in the root folder which describes the options to build and install. Here are some common ones:

```bash
# Build the controller manager binary
make manager

# Run the tests
make test

# Generate manifests e.g. CRD, RBAC YAML files etc
make manifests
```

If you want to start Triton-controller-manager locally to work with a Kubernetes cluster, you should follow the [debug guide](./docs/debug/README.md).

## Engage to help anything

We choose GitHub as the primary place for Triton to collaborate.
So the latest updates of Triton are always here.
Although contributions via PR is an explicit way to help, we still call for any other ways.

- reply to other's issues if you could;
- help solve other user's problems;
- help review other's PR design;
- help review other's codes in PR;
- discuss about Triton to make things clearer;
- advocate Triton technology beyond GitHub;
- write blogs on Triton and so on.

In a word, **ANY HELP IS CONTRIBUTION**.

## Join Triton as a member

It is also welcomed to join Triton team if you are willing to participate in Triton community continuously and keep active.

### Requirements

- Have read the [Contributing to Triton](./CONTRIBUTING.md) carefully
- Have read the [Contributor Covenant Code of Conduct](./CODE_OF_CONDUCT.md)
- Have submitted multi PRs to the community
- Be active in the community, may including but not limited
  - Submitting or commenting on issues
  - Contributing PRs to the community
  - Reviewing PRs in the community

### How to do it

You can do it in either of two ways:

- Submit a PR in the project repo
- Contact via the [community](./README.md#community) channels offline
