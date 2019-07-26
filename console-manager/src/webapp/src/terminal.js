import {Terminal} from 'xterm';
import * as fit from 'xterm/lib/addons/fit/fit';
import * as attach from 'xterm/lib/addons/attach/attach';
import * as fullscreen from 'xterm/lib/addons/fullscreen/fullscreen';
import * as search from 'xterm/lib/addons/search/search';
import * as webLinks from 'xterm/lib/addons/webLinks/webLinks';
import * as winptyCompat from 'xterm/lib/addons/winptyCompat/winptyCompat';
import 'xterm/dist/xterm.css';
import './style.css';
import 'xterm/dist/addons/fullscreen/fullscreen.css';


Terminal.applyAddon(attach);
Terminal.applyAddon(fit);
Terminal.applyAddon(fullscreen);
Terminal.applyAddon(search);
Terminal.applyAddon(webLinks);
Terminal.applyAddon(winptyCompat);

const div = document.createElement('div');
div.innerHTML = '<a id="reload" href="#">back</a><div id="terminal-container"></div>';
document.body.appendChild(div);

document.getElementById('reload').onclick = () => {
    location.reload();
    return false;
};

let terminalId;

let term,
    protocol,
    socketURL,
    socket;

let terminalContainer = document.getElementById('terminal-container');

fetch('/create-terminal/' + window.agentId)
    .then(res => res.text())
    .then(res => {
        terminalId = res;
        createTerminal();
    });

function createTerminal() {
    while (terminalContainer.children.length) {
        terminalContainer.removeChild(terminalContainer.children[0]);
    }

    term = new Terminal({});
    window.term = term;

    protocol = (location.protocol === 'https:') ? 'wss://' : 'ws://';
    socketURL = () => (protocol + location.hostname + ((location.port) ? (':' + location.port) : '') + '/terminal/' + terminalId);

    term.open(terminalContainer);
    term.winptyCompatInit();
    term.webLinksInit();
    term.fit();
    term.focus();

    setTimeout(function () {
        updateTerminalSize();
        fetch('/terminal-config?cols=' + term.cols + '&rows=' + term.rows + '&id=' + terminalId, {method: 'POST'})
            .then(res => res.text())
            .then(() => {
                socket = new WebSocket(socketURL());
                socket.onopen = runRealTerminal;
                socket.onclose = runFakeTerminal;
                socket.onerror = runFakeTerminal;

                term.on('resize', function (size) {
                    let cols = size.cols,
                        rows = size.rows,
                        url = '/terminal-config?cols=' + cols + '&rows=' + rows + '&id=' + terminalId;

                    fetch(url, {method: 'POST'})
                        .then(res => res.text())
                        .then(res => {
                            terminalId = res;
                        });
                });
            });
    }, 0);
}

function runRealTerminal() {
    term.attach(socket);
    term._initialized = true;
}

function runFakeTerminal() {
    if (term._initialized) {
        return;
    }

    term._initialized = true;

    let shellPrompt = '$ ';

    term.prompt = function () {
        term.write('\r\n' + shellPrompt);
    };

    term.writeln('Welcome to xterm.js');
    term.writeln('This is a local terminal emulation, without a real terminal in the back-end.');
    term.writeln('Type some keys and commands to play around.');
    term.writeln('');
    term.prompt();

    term._core.register(term.addDisposableListener('key', function (key, ev) {
        let printable = (
            !ev.altKey && !ev.altGraphKey && !ev.ctrlKey && !ev.metaKey
        );

        if (ev.keyCode === 13) {
            term.prompt();
        } else if (ev.keyCode === 8) {
            if (term.x > 2) {
                term.write('\b \b');
            }
        } else if (printable) {
            term.write(key);
        }
    }));

    term._core.register(term.addDisposableListener('paste', function (data) {
        term.write(data);
    }));
}

function updateTerminalSize() {
    let cols = parseInt(term.cols, 10);
    let rows = parseInt(term.rows, 10);
    let width = (cols * term._core.renderer.dimensions.actualCellWidth + term._core.viewport.scrollBarWidth).toString() + 'px';
    let height = (rows * term._core.renderer.dimensions.actualCellHeight).toString() + 'px';
    terminalContainer.style.width = width;
    terminalContainer.style.height = height;
    term.fit();
}