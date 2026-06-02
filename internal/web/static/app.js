(function () {
    'use strict';

    /* ======================================================================
       Constants
       ====================================================================== */

    var SNAPSHOT_INTERVAL = 2000;
    var WS_RECONNECT_DELAY = 3000;
    var RESTART_RELOAD_DELAY = 3000;

    /* Must match camera.ParamRanges keys (PascalCase ONVIF names) */
    var IMAGING_SLIDERS = [
        { name: 'Brightness', label: 'Brightness', min: -1, max: 1, step: 0.1 },
        { name: 'Contrast', label: 'Contrast', min: 0, max: 32, step: 0.5 },
        { name: 'Saturation', label: 'Saturation', min: 0, max: 32, step: 0.5 },
        { name: 'Sharpness', label: 'Sharpness', min: 0, max: 16, step: 0.5 }
    ];

    var AWB_MODES = ['auto', 'incandescent', 'tungsten', 'fluorescent', 'daylight', 'cloudy', 'custom'];
    var EXPOSURE_MODES = ['normal', 'sport', 'short', 'long', 'custom'];

    /* ======================================================================
       State
       ====================================================================== */

    var snapshotTimer = null;
    var ws = null;
    var wsReconnectTimer = null;

    /* ======================================================================
       DOM helpers
       ====================================================================== */

    function $(sel) { return document.querySelector(sel); }
    function $$(sel) { return document.querySelectorAll(sel); }

    function el(tag, attrs, children) {
        var e = document.createElement(tag);
        if (attrs) {
            Object.keys(attrs).forEach(function (k) {
                if (k === 'className') { e.className = attrs[k]; }
                else if (k === 'textContent') { e.textContent = attrs[k]; }
                else if (k === 'innerHTML') { e.innerHTML = attrs[k]; }
                else if (k.indexOf('on') === 0) { e.addEventListener(k.slice(2).toLowerCase(), attrs[k]); }
                else { e.setAttribute(k, attrs[k]); }
            });
        }
        if (children) {
            (Array.isArray(children) ? children : [children]).forEach(function (c) {
                if (typeof c === 'string') { e.appendChild(document.createTextNode(c)); }
                else if (c) { e.appendChild(c); }
            });
        }
        return e;
    }

    /* ======================================================================
       Toast
       ====================================================================== */

    var toastTimer = null;

    function showToast(msg, duration) {
        var t = $('#toast');
        t.textContent = msg;
        t.classList.remove('hidden');
        clearTimeout(toastTimer);
        toastTimer = setTimeout(function () { t.classList.add('hidden'); }, duration || 4000);
    }

    /* ======================================================================
       Tabs
       ====================================================================== */

    function initTabs() {
        $$('.tab').forEach(function (btn) {
            btn.addEventListener('click', function () {
                $$('.tab').forEach(function (b) { b.classList.remove('active'); });
                $$('.tab-panel').forEach(function (p) { p.classList.remove('active'); });
                btn.classList.add('active');
                var panel = $('#tab-' + btn.dataset.tab);
                if (panel) { panel.classList.add('active'); }
            });
        });
    }

    /* ======================================================================
       API helpers
       ====================================================================== */

    function api(method, url, body) {
        var opts = { method: method, headers: {} };
        if (body !== undefined) {
            opts.headers['Content-Type'] = 'application/json';
            opts.body = JSON.stringify(body);
        }
        return fetch(url, opts).then(function (r) {
            if (!r.ok) {
                return r.text().then(function (t) {
                    throw new Error(t || r.statusText);
                });
            }
            var ct = r.headers.get('content-type') || '';
            if (ct.indexOf('json') !== -1) { return r.json(); }
            return null;
        });
    }

    /* ======================================================================
       Server Tab — Config Display
       ====================================================================== */

    function loadConfig() {
        api('GET', '/api/config').then(function (data) {
            renderConfig(data);
        }).catch(function (err) {
            showToast('Failed to load config: ' + err.message);
        });
    }

    function renderConfig(data) {
        var grid = $('#server-config');
        grid.innerHTML = '';

        var sections = ['camera', 'rtsp', 'onvif', 'rtmp', 'device', 'logging', 'web'];
        sections.forEach(function (sec) {
            var obj = data[sec];
            if (!obj || typeof obj !== 'object') { return; }

            var card = el('div', { className: 'card' });
            card.appendChild(el('h2', { className: 'card-title', textContent: sec.toUpperCase() }));

            Object.keys(obj).forEach(function (key) {
                var val = obj[key];
                if (val === null || val === undefined) { val = ''; }
                if (typeof val === 'object') { val = JSON.stringify(val); }

                var row = el('div', { className: 'config-row' });
                row.appendChild(el('span', { className: 'config-key', textContent: key }));
                var valSpan = el('span', { className: 'config-val mono' });

                if (key === 'password') {
                    valSpan.textContent = '\u2022'.repeat(String(val).length || 8);
                } else {
                    valSpan.textContent = String(val);
                }
                row.appendChild(valSpan);
                card.appendChild(row);
            });

            grid.appendChild(card);
        });
    }

    /* ======================================================================
       ONVIF Modal
       ====================================================================== */

    function initOnvifModal() {
        var overlay = $('#modal-overlay');
        var btnEdit = $('#btn-edit-onvif');
        var btnSave = $('#btn-save-onvif');
        var btnCancel = $('#btn-cancel-onvif');
        var errBox = $('#modal-error');
        var inputUser = $('#input-onvif-user');
        var inputPass = $('#input-onvif-pass');

        function openModal() {
            errBox.classList.add('hidden');
            inputUser.value = '';
            inputPass.value = '';
            overlay.classList.remove('hidden');
            inputUser.focus();
        }

        function closeModal() {
            overlay.classList.add('hidden');
        }

        btnEdit.addEventListener('click', openModal);
        btnCancel.addEventListener('click', closeModal);

        overlay.addEventListener('click', function (e) {
            if (e.target === overlay) { closeModal(); }
        });

        document.addEventListener('keydown', function (e) {
            if (e.key === 'Escape' && !overlay.classList.contains('hidden')) {
                closeModal();
            }
        });

        btnSave.addEventListener('click', function () {
            var username = inputUser.value.trim();
            var password = inputPass.value;

            if (!username) {
                errBox.textContent = 'Username is required';
                errBox.classList.remove('hidden');
                return;
            }

            btnSave.disabled = true;
            btnSave.textContent = 'Saving...';

            api('POST', '/api/config/onvif', { username: username, password: password })
                .then(function () {
                    closeModal();
                    showRestartBanner();
                })
                .catch(function (err) {
                    errBox.textContent = err.message;
                    errBox.classList.remove('hidden');
                })
                .finally(function () {
                    btnSave.disabled = false;
                    btnSave.textContent = 'Save & Restart';
                });
        });
    }

    function showRestartBanner() {
        $('#restart-banner').classList.remove('hidden');
        setTimeout(function () {
            window.location.reload();
        }, RESTART_RELOAD_DELAY);
    }

    /* ======================================================================
       Camera Tab — Snapshot
       ====================================================================== */

    function initSnapshot() {
        var img = $('#snapshot');
        var placeholder = $('#snapshot-placeholder');

        img.addEventListener('load', function () {
            img.style.display = 'block';
            placeholder.style.display = 'none';
        });

        img.addEventListener('error', function () {
            img.style.display = 'none';
            placeholder.style.display = 'flex';
        });

        refreshSnapshot();
        snapshotTimer = setInterval(refreshSnapshot, SNAPSHOT_INTERVAL);
    }

    function refreshSnapshot() {
        var img = $('#snapshot');
        img.src = '/api/snapshot?ts=' + Date.now();
    }

    /* ======================================================================
       Camera Tab — Imaging Controls
       ====================================================================== */

    function loadImaging() {
        Promise.all([
            api('GET', '/api/camera/params'),
            api('GET', '/api/camera/options')
        ]).then(function (results) {
            var params = results[0] || {};
            var options = results[1] || {};
            renderImaging(params, options);
        }).catch(function (err) {
            showToast('Failed to load imaging controls: ' + err.message);
        });
    }

    function renderImaging(params, options) {
        var container = $('#imaging-controls');
        container.innerHTML = '';

        /* Sliders */
        IMAGING_SLIDERS.forEach(function (cfg) {
            var current = params[cfg.name] !== undefined ? Number(params[cfg.name]) : 0;
            var range = options[cfg.name] || {};
            var min = range.min !== undefined ? range.min : cfg.min;
            var max = range.max !== undefined ? range.max : cfg.max;
            var step = range.step !== undefined ? range.step : cfg.step;

            var wrap = el('div', { className: 'param-control' });

            var header = el('div', { className: 'param-header' });
            header.appendChild(el('span', { className: 'param-label', textContent: cfg.label }));
            var valSpan = el('span', { className: 'param-value mono', textContent: current.toFixed(1) });
            header.appendChild(valSpan);
            wrap.appendChild(header);

            var slider = el('input', {
                className: 'param-slider',
                type: 'range',
                min: String(min),
                max: String(max),
                step: String(step),
                value: String(current),
                'data-param': cfg.name
            });

            slider.addEventListener('input', function () {
                valSpan.textContent = Number(slider.value).toFixed(1);
            });

            slider.addEventListener('change', function () {
                var v = Number(slider.value);
                valSpan.textContent = v.toFixed(1);
                postParam(cfg.name, v);
                flashValue(wrap);
            });

            wrap.appendChild(slider);

            var labels = el('div', { className: 'param-range-labels' });
            labels.appendChild(el('span', { textContent: min }));
            labels.appendChild(el('span', { textContent: max }));
            wrap.appendChild(labels);

            container.appendChild(wrap);
        });

        /* AWB Mode — param name is "AWBMode" (PascalCase, matches ParamEnums key) */
        var awbVal = params.AWBMode || 'auto';
        var awbEnums = (options.AWBMode && options.AWBMode.enums) || AWB_MODES;
        container.appendChild(buildSelect('AWB Mode', 'AWBMode', awbVal, awbEnums));

        /* Exposure Mode — param name is "ExposureMode" */
        var expVal = params.ExposureMode || 'normal';
        var expEnums = (options.ExposureMode && options.ExposureMode.enums) || EXPOSURE_MODES;
        container.appendChild(buildSelect('Exposure Mode', 'ExposureMode', expVal, expEnums));

        /* Boolean toggles — param names are "HFlip" and "VFlip" (PascalCase) */
        var bools = [
            { name: 'HFlip', label: 'Horizontal Flip' },
            { name: 'VFlip', label: 'Vertical Flip' }
        ];

        bools.forEach(function (b) {
            var on = !!params[b.name];
            var row = el('div', { className: 'param-control param-bool' });
            row.appendChild(el('span', { className: 'param-label', textContent: b.label }));

            var toggle = el('label', { className: 'toggle' });
            var input = el('input', { type: 'checkbox', 'data-param': b.name });
            if (on) { input.checked = true; }

            input.addEventListener('change', function () {
                postParam(b.name, input.checked);
            });

            toggle.appendChild(input);
            toggle.appendChild(el('span', { className: 'toggle-slider' }));
            row.appendChild(toggle);
            container.appendChild(row);
        });
    }

    function buildSelect(label, name, current, enums) {
        var wrap = el('div', { className: 'param-control' });
        wrap.appendChild(el('span', { className: 'param-label', textContent: label }));

        var sel = el('select', { className: 'param-select', 'data-param': name });
        enums.forEach(function (opt) {
            var o = el('option', { value: opt, textContent: opt });
            if (opt === current) { o.selected = true; }
            sel.appendChild(o);
        });

        sel.addEventListener('change', function () {
            postParam(name, sel.value);
        });

        wrap.appendChild(sel);
        return wrap;
    }

    function postParam(name, value) {
        api('POST', '/api/camera/param', { name: name, value: value }).catch(function (err) {
            showToast('Param error (' + name + '): ' + err.message);
        });
    }

    function flashValue(wrap) {
        wrap.classList.add('flash');
        setTimeout(function () { wrap.classList.remove('flash'); }, 400);
    }

    /* ======================================================================
       Camera Tab — PTZ Controls
       ====================================================================== */

    function loadPTZ() {
        api('GET', '/api/ptz/status').then(function (data) {
            updatePTZDisplay(data);
        }).catch(function () { /* will get updates via WS */ });

        loadPresets();
    }

    function updatePTZDisplay(data) {
        if (!data) { return; }
        var pos = data.position || data;
        /* Go struct fields: Pan, Tilt, Zoom (capitalized) */
        var pan = pos.Pan !== undefined ? pos.Pan : 0;
        var tilt = pos.Tilt !== undefined ? pos.Tilt : 0;
        var zoom = pos.Zoom !== undefined ? pos.Zoom : 0;

        $('#ptz-pan').textContent = pan.toFixed(3);
        $('#ptz-tilt').textContent = tilt.toFixed(3);
        $('#ptz-zoom').textContent = zoom.toFixed(3);

        if (data.status) {
            $('#ptz-status').textContent = data.status;
        }
    }

    function initPTZControls() {
        /* Directions map to ptz.Velocity fields: Pan, Tilt, Zoom */
        var dirs = {
            up: { Pan: 0, Tilt: 0.5, Zoom: 0 },
            down: { Pan: 0, Tilt: -0.5, Zoom: 0 },
            left: { Pan: -0.5, Tilt: 0, Zoom: 0 },
            right: { Pan: 0.5, Tilt: 0, Zoom: 0 }
        };

        $$('.dpad-btn[data-dir]').forEach(function (btn) {
            var dir = btn.dataset.dir;
            if (!dirs[dir]) { return; }

            btn.addEventListener('mousedown', function (e) {
                e.preventDefault();
                startMove(dirs[dir]);
            });
            btn.addEventListener('mouseup', function () { stopMove(); });
            btn.addEventListener('mouseleave', function () { stopMove(); });

            btn.addEventListener('touchstart', function (e) {
                e.preventDefault();
                startMove(dirs[dir]);
            }, { passive: false });
            btn.addEventListener('touchend', function () { stopMove(); });
            btn.addEventListener('touchcancel', function () { stopMove(); });
        });

        /* Stop button */
        var stopBtn = $('#btn-ptz-stop');
        stopBtn.addEventListener('mousedown', function (e) { e.preventDefault(); });
        stopBtn.addEventListener('click', function () {
            stopMove();
        });

        /* Zoom buttons */
        var zoomIn = $('[data-dir="zoom-in"]');
        var zoomOut = $('[data-dir="zoom-out"]');

        if (zoomIn) {
            zoomIn.addEventListener('mousedown', function (e) {
                e.preventDefault();
                startMove({ Pan: 0, Tilt: 0, Zoom: 0.3 });
            });
            zoomIn.addEventListener('mouseup', function () { stopMove(); });
            zoomIn.addEventListener('mouseleave', function () { stopMove(); });
            zoomIn.addEventListener('touchstart', function (e) {
                e.preventDefault();
                startMove({ Pan: 0, Tilt: 0, Zoom: 0.3 });
            }, { passive: false });
            zoomIn.addEventListener('touchend', function () { stopMove(); });
            zoomIn.addEventListener('touchcancel', function () { stopMove(); });
        }

        if (zoomOut) {
            zoomOut.addEventListener('mousedown', function (e) {
                e.preventDefault();
                startMove({ Pan: 0, Tilt: 0, Zoom: -0.3 });
            });
            zoomOut.addEventListener('mouseup', function () { stopMove(); });
            zoomOut.addEventListener('mouseleave', function () { stopMove(); });
            zoomOut.addEventListener('touchstart', function (e) {
                e.preventDefault();
                startMove({ Pan: 0, Tilt: 0, Zoom: -0.3 });
            }, { passive: false });
            zoomOut.addEventListener('touchend', function () { stopMove(); });
            zoomOut.addEventListener('touchcancel', function () { stopMove(); });
        }
    }

    function startMove(velocity) {
        api('POST', '/api/ptz/move', velocity)
            .catch(function (err) { showToast('PTZ move error: ' + err.message); });
    }

    function stopMove() {
        api('POST', '/api/ptz/stop').catch(function () { /* best effort */ });
    }

    /* ======================================================================
       Camera Tab — Presets
       ====================================================================== */

    function loadPresets() {
        api('GET', '/api/ptz/presets').then(function (data) {
            renderPresets(data);
        }).catch(function () {
            renderPresets([]);
        });
    }

    function renderPresets(presets) {
        var container = $('#preset-list');
        container.innerHTML = '';

        if (!presets || !presets.length) {
            container.appendChild(el('p', { className: 'empty-msg', textContent: 'No presets defined' }));
            return;
        }

        presets.forEach(function (p) {
            var row = el('div', { className: 'preset-row' });

            var info = el('div', { className: 'preset-info' });
            info.appendChild(el('span', { className: 'preset-name', textContent: p.name || ('Preset ' + p.token) }));

            var posText = 'Token: ' + p.token;
            if (p.position) {
                var pan = (p.position.Pan || 0).toFixed(2);
                var tilt = (p.position.Tilt || 0).toFixed(2);
                var zoom = (p.position.Zoom || 0).toFixed(2);
                posText += '  |  P:' + pan + ' T:' + tilt + ' Z:' + zoom;
            }
            info.appendChild(el('span', { className: 'preset-pos mono', textContent: posText }));
            row.appendChild(info);

            var actions = el('div', { className: 'preset-actions' });
            actions.appendChild(el('button', {
                className: 'btn btn-sm',
                textContent: 'Goto',
                onClick: function () { gotoPreset(p.token); }
            }));
            actions.appendChild(el('button', {
                className: 'btn btn-sm btn-danger',
                textContent: 'Delete',
                onClick: function () { deletePreset(p.token); }
            }));
            row.appendChild(actions);

            container.appendChild(row);
        });
    }

    function addPreset() {
        var count = $$('#preset-list .preset-row').length;
        api('POST', '/api/ptz/preset', { name: 'Preset ' + (count + 1) })
            .then(function () { loadPresets(); })
            .catch(function (err) { showToast('Add preset error: ' + err.message); });
    }

    function gotoPreset(token) {
        api('POST', '/api/ptz/preset/goto', { token: token })
            .catch(function (err) { showToast('Goto preset error: ' + err.message); });
    }

    function deletePreset(token) {
        api('DELETE', '/api/ptz/preset/' + encodeURIComponent(token))
            .then(function () { loadPresets(); })
            .catch(function (err) { showToast('Delete preset error: ' + err.message); });
    }

    function initPresetControls() {
        $('#btn-add-preset').addEventListener('click', addPreset);
    }

    /* ======================================================================
       WebSocket
       ====================================================================== */

    function connectWS() {
        var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        var url = proto + '//' + location.host + '/ws';

        try {
            ws = new WebSocket(url);
        } catch (e) {
            scheduleReconnect();
            return;
        }

        ws.onopen = function () {
            setWSStatus(true);
        };

        ws.onclose = function () {
            setWSStatus(false);
            scheduleReconnect();
        };

        ws.onerror = function () {
            /* onclose will fire after this */
        };

        ws.onmessage = function (evt) {
            handleWSMessage(evt.data);
        };
    }

    function scheduleReconnect() {
        clearTimeout(wsReconnectTimer);
        wsReconnectTimer = setTimeout(connectWS, WS_RECONNECT_DELAY);
    }

    function setWSStatus(connected) {
        var badge = $('#ws-status');
        if (connected) {
            badge.textContent = 'Connected';
            badge.className = 'status-badge connected';
        } else {
            badge.textContent = 'Disconnected';
            badge.className = 'status-badge disconnected';
        }
    }

    function handleWSMessage(raw) {
        var msg;
        try {
            msg = JSON.parse(raw);
        } catch (e) { return; }

        switch (msg.type) {
            case 'ping':
                break;

            case 'param-changed':
                /* msg.name and msg.value at top level (wsEvent struct) */
                applyParamUpdate(msg.name, msg.value);
                break;

            case 'ptz-position':
                /*
                 * Initial state sends: {"type":"ptz-position","position":{"Pan":0,"Tilt":0,"Zoom":0}}
                 * Ongoing hook sends: {"type":"ptz-position"} (no position data)
                 * Handle both — re-fetch if position missing.
                 */
                if (msg.position && msg.position.Pan !== undefined) {
                    updatePTZDisplay(msg);
                } else {
                    fetchPTZStatus();
                }
                break;

            case 'preset-list-changed':
                loadPresets();
                break;
        }
    }

    function fetchPTZStatus() {
        api('GET', '/api/ptz/status').then(function (data) {
            updatePTZDisplay(data);
        }).catch(function () { /* silent */ });
    }

    function applyParamUpdate(name, value) {
        if (!name) { return; }

        /* Try slider */
        var slider = $('.param-slider[data-param="' + name + '"]');
        if (slider) {
            slider.value = value;
            var valSpan = slider.parentElement.querySelector('.param-value');
            if (valSpan) {
                valSpan.textContent = Number(value).toFixed(1);
            }
            flashValue(slider.parentElement);
            return;
        }

        /* Try select */
        var sel = $('.param-select[data-param="' + name + '"]');
        if (sel) {
            sel.value = value;
            return;
        }

        /* Try checkbox */
        var cb = $('input[type="checkbox"][data-param="' + name + '"]');
        if (cb) {
            cb.checked = !!value;
        }
    }

    /* ======================================================================
       Init
       ====================================================================== */

    function init() {
        initTabs();
        initOnvifModal();
        initPTZControls();
        initPresetControls();

        /* Server tab */
        loadConfig();

        /* Camera tab */
        initSnapshot();
        loadImaging();
        loadPTZ();

        /* WebSocket */
        connectWS();
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

})();
