{ pkgs, ... }: {

  channel = "stable-24.11";
  packages = [
    pkgs.nodejs_20
    pkgs.pnpm
    pkgs.fish
    pkgs.fastfetch
    pkgs.htop
    pkgs.openssl
  ];
  services.postgres = {
    enable = true;
  };
  services.redis = {
    enable = true;
  };
  idx = {
    extensions = [
      "bradlc.vscode-tailwindcss"
      "dbaeumer.vscode-eslint"
      "EditorConfig.EditorConfig"
      "esbenp.prettier-vscode"
      "mgmcdermott.vscode-language-babel"
      "streetsidesoftware.code-spell-checker"
      "stylelint.vscode-stylelint"
      "syler.sass-indented"
      "zhuangtongfa.material-theme"
    ];
    # Workspace lifecycle hooks
    workspace = {
      # Runs when a workspace is first created
      onCreate = {
        # Example: install JS dependencies from NPM
        # npm-install = "npm install";
        "setup" = "pnpm i; cp .env.example .env; pnpm dev;";
      };
      # Runs when the workspace is (re)started
      onStart = {
        # Example: start a background task to watch and re-build backend code
        # watch-backend = "npm run watch-backend";
        "setup" = "pnpm run dev";
      };
    };
  };
}
