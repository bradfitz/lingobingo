function hi() {
    alert("hello");
}

console.log("init.");

var ws = null;

function cell(row, col) {
    var board = document.getElementById("board");
    var tr = board.getElementsByTagName("tr")[row + 1];
    return tr.getElementsByTagName("td")[col];
}

function setHint(e, v) {
    if (v) {
        e.classList.add("hint");
    } else {
        e.classList.remove("hint");
    }
}

function setMarked(e, v) {
    if (v) {
        e.classList.add("marked");
        e.classList.remove("hint");
    } else {
        e.classList.remove("marked");
    }
}

function onBingo(winLine) {
    var tds = document.getElementsByTagName("td")
    for (var i = 0; i < tds.length; i++) {
        var e = tds[i];
        if (e.classList.contains("hint")) {
            setMarked(e, true);
        }
    }
    for (var i = 0; i < winLine.length; i++) {
        var e = cell(winLine[i][0], winLine[i][1]);
        e.classList.remove("marked");
        e.classList.add("win");
    }

}

function isMarked(e) {
    return e.classList.contains("marked");
}

document.addEventListener("DOMContentLoaded", function (event) {
    console.log("DOMContentLoaded");
    connect();
    var tds = document.getElementsByTagName("td")
    for (var row = 0; row < 5; row++) {
        for (var col = 0; col < 5; col++) {
            if (row == 2 && col == 2) {
                continue;
            }
            var e = cell(row, col);
            var isTouch = ("ontouchstart" in window);
            e.addEventListener(isTouch ? "touchstart" : "mousedown", function (ev) {
                var e = ev.target;
                setMarked(e, !isMarked(e))
            }, true);
        }
    }
});


function connect() {
    if (ws != null) {
        ws.close();
        ws = null;
    }
    console.log("connecting...");
    ws = new WebSocket("wss://play.bingo.ts.net/");
    ws.onopen = function (e) {
        console.log("connected.");
    };
    ws.onclose = function (e) {
        console.log("closed.");
        window.setTimeout(connect, 1000);
    };
    ws.onmessage = function (e) { eval(e.data) };
}
