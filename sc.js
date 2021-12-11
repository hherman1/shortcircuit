    // looks up the node at the given ipath
    function get(path) {
        var cur = document
        for (let i of path) {
            cur = cur.childNodes[i]
        }
        return cur
    }
window.onload = function () {

    var conn;
    conn = new WebSocket("ws://localhost:8080/ws");
    conn.onmessage = function (evt) {
        var messages = evt.data.split('\n');
        for (var i = 0; i < messages.length; i++) {
            var message = messages[i]
            message = message.trim()
            if (message === "") {
                continue;
            }
            let changes = JSON.parse(message)
            for (let change of changes) {
                let p = get(change.IPath)
                if (change.InsertNode != null) {
                    if (p.childNodes.length == 0) {
                        p.innerHTML = change.InsertNode.Html;
                        continue;
                    }
                    let container = document.createElement("div")
                    container.innerHTML = change.InsertNode.Html
                    let parsed = container.childNodes[0]
                    // parse node
                    if (change.InsertNode.Index >= p.childNodes.length) {
                        p.appendChild(parsed)
                        continue;
                    }
                    let child = p.childNodes[change.InsertNode.Index];
                    p.insertBefore(parsed, child)
                    continue;
                } else if (change.Rmnode != null) {
                    p.childNodes[change.Rmnode].remove()
                    continue;
                }
            }
        }
    }

    function clickEvent(e) {
        conn.send(JSON.stringify({"Type": "click", "Message": e.target.getAttribute("sc-click")}));
    }

    // Setup event listeners for children of the given node
    function registerListeners(node) {
        for (let el of node.querySelectorAll("[sc-click]")) {
            el.addEventListener("click", clickEvent);
        }
    }


    registerListeners(document);

}