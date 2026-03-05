# Sideterm for Emacs

Sideterm communicates with i3 and kitty to make sure that we're always showing a terminal tab for the project that we currently have open in emacs. I tried running the different terminal emulators that are available for emacs, but always ran into fiddly hard corners, so now I'm gluing kitty to emacs :). Needless to say this is a very specialised tool for my specific environment setup.

## The flow

My emacs config includes this little snippet for exposing the project name and path in the window title:

``` emacs-lisp
(setq frame-title-format
      '(:eval (let ((tab-name (alist-get 'name (tab-bar--current-tab)))
                    (proj (project-current)))
                (if proj
                    (format "%s - %s" tab-name (project-root proj))
                  tab-name))))
```

Here already things start to get very customised to my emacs setup, I use [one-tab-per-project](https://github.com/abougouffa/one-tab-per-project) to make it easier to work with a bunch of things in parallel (hence my need to swap terminals in sync with switching projects).

Then I start kitty with remote control over a unix socket: `kitty -o allow_remote_control=yes -o enabled_layouts=tall -o listen_on unix:@emacs` and place that terminal next to emacs. I use that initial terminal window to start sideterm.

Sideterm opens an IPC connection to i3 to listen for Emacs window title changes. When it sees a title matching the pattern of project name + path it makes sure that kitty has a tab named after the project with two terminal windows in a horizontal split.
