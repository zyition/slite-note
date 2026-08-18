# Third-Party Notices

slite-note is built on the following open-source projects. Licenses are
reproduced in full where required.

## Go dependencies

| Project | License | Purpose |
|---|---|---|
| [Wails v3](https://github.com/wailsapp/wails) | [MPL-2.0](https://www.mozilla.org/en-US/MPL/2.0/) | Desktop shell (window/tray/hotkey/bindings) |
| [golang.org/x/sys](https://go.googlesource.com/sys) | [BSD-3-Clause](https://github.com/golang/sys/blob/master/LICENSE) | Win32 interop |
| [github.com/adrg/xdg](https://github.com/adrg/xdg) | [MIT](https://github.com/adrg/xdg/blob/master/LICENSE) | User dirs |
| [github.com/coder/websocket](https://github.com/coder/websocket) | [ISC](https://github.com/coder/websocket/blob/main/LICENSE.txt) | Wails IPC |
| [github.com/go-ole/go-ole](https://github.com/go-ole/go-ole) | [MIT](https://github.com/go-ole/go-ole/blob/master/LICENSE) | COM interop |
| [github.com/jchv/go-winloader](https://github.com/jchv/go-winloader) | [MIT](https://github.com/jchv/go-winloader/blob/master/LICENSE) | WebView2 loader |

## Frontend dependencies

| Project | License | Purpose |
|---|---|---|
| [React](https://github.com/facebook/react) | [MIT](https://github.com/facebook/react/blob/main/LICENSE) | UI framework |
| [Vite](https://github.com/vitejs/vite) | [MIT](https://github.com/vitejs/vite/blob/main/LICENSE) | Build tool |
| [TypeScript](https://github.com/microsoft/TypeScript) | [Apache-2.0](https://github.com/microsoft/TypeScript/blob/main/LICENSE.txt) | Language |
| [Tailwind CSS v4](https://github.com/tailwindlabs/tailwindcss) | [MIT](https://github.com/tailwindlabs/tailwindcss/blob/main/LICENSE) | Styling |
| [BlockNote](https://github.com/TypeCellOS/BlockNote) | [MPL-2.0](https://github.com/TypeCellOS/BlockNote/blob/main/LICENSE) | Block editor |
| [lucide-react](https://github.com/lucide-icons/lucide) | [ISC](https://github.com/lucide-icons/lucide/blob/main/LICENSE) | Icons |
| [Mantine](https://github.com/mantinedev/mantine) (via BlockNote) | [MIT](https://github.com/mantinedev/mantine/blob/master/LICENSE) | Editor menus |

Full license texts for MPL-2.0 dependencies:

- **Mozilla Public License Version 2.0** — reproduced at
  <https://www.mozilla.org/en-US/MPL/2.0/>.
  Wails (including its `wails_windows_*.syso` build assets and the WebView2
  bootstrap) and BlockNote are distributed under the MPL-2.0; this does not
  affect the license of slite-note's own code.

The WebView2 runtime is a Microsoft product distributed under the
[Microsoft Edge WebView2 Runtime license](https://aka.ms/WebView2BrowserExtensionLicense).
