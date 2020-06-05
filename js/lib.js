/**
 * @return {string}
 */
function base64_encode(buffer) {
    let buf = [];
    let bytes = new Uint8Array(buffer);
    let len = bytes.byteLength;
    for (let i = 0; i < len; i++) {
        buf[i] = String.fromCharCode(bytes[i]);
    }
    return window.btoa(buf.join(""));
}


function base64_decode(data) {
    const decoded = atob(data);
    const n = decoded.length;
    let bytes = new Uint8Array(n);
    for (let i = 0; i < n; i++) {
        bytes[i] = decoded.charCodeAt(i);
    }
    return new Blob([bytes]);
}

async function make_request(method, url, headers, data) {
    method = method.toUpperCase();

    let args = {
        method: method,
        cache: 'no-cache',
        headers: headers,
    };
    if (method !== "GET" && method !== "HEAD" && method !== "OPTIONS") {
        args["body"] = base64_decode(data);
    }

    let res = await fetch(url, args);
    return {
        "status": res.status,
        "body": base64_encode(await res.arrayBuffer()),
        "headers": Object.fromEntries(res.headers.entries())
    };
}
