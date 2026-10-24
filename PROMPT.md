# Project prompt

New project, I want you to build a new go cli tool for managing .env credential files. 

Use bubbletea for the TUI. 

Projects are the child directories of one or more root directories which can be configured (I might select `['/Users/martin/Herd', '/User/martin/Developer'] for managing my Laravel apps and other non-Laravel projects). Projects should initially be named after the name of the project directory. The path and name of the project should be editable.

The user should be able to store different .env files for each of the projects (for example "Bob's Todo App", "Staging Todo App", "Production App" etc... might be names of .env files for the project "Todo App".

The tool should remember which stored .env file is currently 'selected' (meaning it is the current contents of actual .env file in the project directory). I should be able to switch to different .env files using a command which is aware of the current working directory as the project directory.

Each store .env file is version controlled using a simple diff and version numbers (v1, v2, v3, v4 ...). The user should be able to "update the file" by making a manual change to the .env file in the project directory, run a command, the app should check if the actual file .env in the project directory has different contents from the same last stored version and if so, it should store a new latest version of that .env file.

After making a significant manual change to the actual .env file in the project directory, I should also be able to run a command which stores the contents as a new .env file for that project.

The TUI interface should heirarchically allow the user to search for listed projects (prioritising project of current working directory), then search for listed .env files (prioritising current selected .env for that project), then search for the version of the .env file (prioritising latest version), using a sidebar/main contents pane UI. The user should be able to edit, delete, search, comment, uncomment, view diff with last version, lint or validate syntax of a chosen file contents. I should also be able to copy the .env contents for sharing as triple '`' tagged  markdown for sending by Microsoft Teams, as text, as diff, etc...

The tool should also actively file watch discovered .gitignore files in root of project directories  to check for if .env files are covered, the user should be notified if not.

The tool should also actively file watch discovered .env files in root of project directories to check for manual changes, the user should be notified of possible actions "update" (which adds new version of selected stored file), and "new file" (which adds the current contents as a new stored file and switch to that file) and "revert" (which reverts the current contents to the latest stored contents).

You might store all of the data (versions of .env files, patches, databases, however you choose to do it) in a directory in the user's home directory (maybe `~/.envm`).

You should also add this prompt to a markdown file in the project. You should start this project as version 0.1.0. You should use the package name "martinshaw.co/environment-manager". The binary name should be `envm`. License should be GPL 3. Create a new public repo with `master` as default branch. Use a fully featured, useful and simple readme with table of contents. It should be multi platform. You should add a github action which builds the binary and releases with patch notes and a binary for each platform.
