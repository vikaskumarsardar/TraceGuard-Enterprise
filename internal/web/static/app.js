// TraceGuard Enterprise Dashboard Frontend Client
(function () {
  'use strict';

  // Application State
  const state = {
    activeTab: 'dashboard',
    isPaused: false,
    traces: [],
    selectedTrace: null,
    metrics: null,
    ws: null
  };

  // DOM Elements
  const elements = {
    tabs: document.querySelectorAll('.nav-item'),
    tabViews: document.querySelectorAll('.tab-view'),
    tracesTableBody: document.getElementById('traces-table-body'),
    fullTracesBody: document.getElementById('full-traces-body'),
    waterfallContainer: document.getElementById('waterfall-tree-container'),
    waterfallTraceId: document.getElementById('waterfall-trace-id'),
    topologyContainer: document.getElementById('topology-nodes-list'),
    
    // Stats
    statSpans: document.getElementById('stat-spans-val'),
    statTraces: document.getElementById('stat-traces-val'),
    statLatency: document.getElementById('stat-latency-val'),
    statErrors: document.getElementById('stat-errors-val'),
    statErrorFooter: document.getElementById('stat-error-footer'),
    tracesCountBadge: document.getElementById('traces-count-badge'),
    
    // Controls
    pauseBtn: document.getElementById('pause-stream-btn'),
    simulateBtn: document.getElementById('trigger-simulation-btn'),
    searchInput: document.getElementById('global-search-input'),
    filterInput: document.getElementById('traces-filter-input')
  };

  // Initialize Application
  function init() {
    setupNavigation();
    setupControls();
    connectWebSocket();
    fetchInitialData();

    // Poll fallback metrics every 3s
    setInterval(fetchMetrics, 3000);
  }

  // Tab Navigation Setup
  function setupNavigation() {
    elements.tabs.forEach(tab => {
      tab.addEventListener('click', () => {
        const targetTab = tab.getAttribute('data-tab');
        if (!targetTab) return;

        elements.tabs.forEach(t => t.classList.remove('active'));
        elements.tabViews.forEach(v => v.classList.remove('active'));

        tab.classList.add('active');
        const activeView = document.getElementById(`view-${targetTab}`);
        if (activeView) activeView.classList.add('active');

        state.activeTab = targetTab;
      });
    });
  }

  // Control Buttons Setup
  function setupControls() {
    if (elements.pauseBtn) {
      elements.pauseBtn.addEventListener('click', () => {
        state.isPaused = !state.isPaused;
        elements.pauseBtn.textContent = state.isPaused ? 'Resume Stream' : 'Pause Stream';
        elements.pauseBtn.classList.toggle('btn-primary', state.isPaused);
      });
    }

    if (elements.simulateBtn) {
      elements.simulateBtn.addEventListener('click', async () => {
        try {
          elements.simulateBtn.disabled = true;
          elements.simulateBtn.textContent = 'Simulating...';
          await fetch('/api/v1/simulate', { method: 'POST' });
          setTimeout(() => {
            elements.simulateBtn.disabled = false;
            elements.simulateBtn.textContent = '+ Simulate Traffic';
            fetchInitialData();
          }, 600);
        } catch (err) {
          console.error('Failed to trigger simulation:', err);
          elements.simulateBtn.disabled = false;
          elements.simulateBtn.textContent = '+ Simulate Traffic';
        }
      });
    }

    if (elements.searchInput) {
      elements.searchInput.addEventListener('input', (e) => {
        filterTraces(e.target.value);
      });
    }
  }

  // WebSocket Live Connection
  function connectWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;

    state.ws = new WebSocket(wsUrl);

    state.ws.onmessage = (event) => {
      if (state.isPaused) return;

      try {
        const data = JSON.parse(event.data);
        if (data.type === 'span' && data.span) {
          handleIncomingSpan(data.span);
        } else if (data.type === 'metrics' && data.metrics) {
          updateMetricsUI(data.metrics);
        }
      } catch (e) {
        console.error('Error parsing WS message:', e);
      }
    };

    state.ws.onclose = () => {
      // Reconnect after 3s
      setTimeout(connectWebSocket, 3000);
    };
  }

  // Initial Rest Data Fetch
  async function fetchInitialData() {
    try {
      const [tracesRes, metricsRes] = await Promise.all([
        fetch('/api/v1/traces'),
        fetch('/api/v1/metrics')
      ]);

      if (tracesRes.ok) {
        const traces = await tracesRes.json();
        state.traces = traces || [];
        renderTracesFeed();
        renderFullTracesTable();
      }

      if (metricsRes.ok) {
        const metrics = await metricsRes.json();
        updateMetricsUI(metrics);
      }
    } catch (err) {
      console.warn('Backend API initial fetch:', err);
    }
  }

  async function fetchMetrics() {
    try {
      const res = await fetch('/api/v1/metrics');
      if (res.ok) {
        const metrics = await res.json();
        updateMetricsUI(metrics);
      }
    } catch (err) {
      // Ignore polling error
    }
  }

  // Process Incoming Span
  function handleIncomingSpan(span) {
    // Find or create trace tree
    let tree = state.traces.find(t => t.trace_id === span.trace_id);
    if (!tree) {
      tree = {
        trace_id: span.trace_id,
        correlation_id: span.correlation_id,
        root_span: span,
        spans: [span],
        total_duration_ms: span.duration_ms,
        start_time: span.start_time,
        service_count: 1,
        has_errors: span.status === 'ERROR'
      };
      state.traces.unshift(tree);
    } else {
      tree.spans.push(span);
      if (span.status === 'ERROR') tree.has_errors = true;
    }

    renderTracesFeed();
    if (state.selectedTrace && state.selectedTrace.trace_id === tree.trace_id) {
      renderWaterfall(tree);
    }
  }

  // UI Renderers
  function renderTracesFeed() {
    if (!elements.tracesTableBody) return;

    if (state.traces.length === 0) {
      elements.tracesTableBody.innerHTML = '<tr class="empty-row"><td colspan="5">Listening for incoming network packets and eBPF events...</td></tr>';
      return;
    }

    let html = '';
    const recentSpans = [];

    // Flatten recent spans
    state.traces.slice(0, 20).forEach(tree => {
      tree.spans.forEach(s => recentSpans.push({ span: s, tree }));
    });

    recentSpans.slice(0, 15).forEach(({ span, tree }) => {
      const statusClass = span.status === 'ERROR' ? 'status-error' : 'status-ok';
      const protoClass = `proto-${span.protocol || 'HTTP'}`;

      html += `
        <tr data-trace-id="${tree.trace_id}">
          <td><span class="proto-tag ${protoClass}">${span.protocol}</span></td>
          <td>
            <strong>${escapeHtml(span.service_name)}</strong>
            <div style="font-size:0.75rem; color:var(--text-muted);">${escapeHtml(span.name)}</div>
          </td>
          <td><code style="color:var(--accent-cyan);">${escapeHtml(span.correlation_id.substring(0, 18))}...</code></td>
          <td>${span.duration_ms.toFixed(2)} ms</td>
          <td><span class="status-badge ${statusClass}">${span.status}</span></td>
        </tr>
      `;
    });

    elements.tracesTableBody.innerHTML = html;

    // Attach click listeners for Waterfall
    elements.tracesTableBody.querySelectorAll('tr[data-trace-id]').forEach(row => {
      row.addEventListener('click', () => {
        const traceId = row.getAttribute('data-trace-id');
        const tree = state.traces.find(t => t.trace_id === traceId);
        if (tree) {
          state.selectedTrace = tree;
          renderWaterfall(tree);
        }
      });
    });
  }

  function renderFullTracesTable() {
    if (!elements.fullTracesBody) return;

    if (state.traces.length === 0) {
      elements.fullTracesBody.innerHTML = '<tr class="empty-row"><td colspan="6">No registered traces found.</td></tr>';
      return;
    }

    let html = '';
    state.traces.forEach(tree => {
      html += `
        <tr data-trace-id="${tree.trace_id}">
          <td><code>${tree.trace_id.substring(0, 16)}...</code></td>
          <td><code style="color:var(--accent-cyan);">${tree.correlation_id.substring(0, 16)}...</code></td>
          <td><strong>${escapeHtml(tree.root_span?.service_name || 'unknown')}</strong></td>
          <td>${tree.spans.length} spans</td>
          <td>${tree.total_duration_ms.toFixed(2)} ms</td>
          <td>${tree.has_errors ? '<span class="status-badge status-error">HAS ERRORS</span>' : '<span class="status-badge status-ok">CLEAN</span>'}</td>
        </tr>
      `;
    });

    elements.fullTracesBody.innerHTML = html;
  }

  function renderWaterfall(tree) {
    if (!elements.waterfallContainer || !tree) return;

    elements.waterfallTraceId.textContent = `Trace: ${tree.trace_id.substring(0, 14)}...`;

    let html = '';
    const maxDuration = Math.max(...tree.spans.map(s => s.duration_ms), 1.0);

    tree.spans.forEach(span => {
      const widthPct = Math.max((span.duration_ms / maxDuration) * 100, 8);
      const isError = span.status === 'ERROR';

      html += `
        <div class="waterfall-bar-row">
          <div class="waterfall-bar-meta">
            <span>
              <strong style="color:var(--text-primary);">${escapeHtml(span.service_name)}</strong> 
              <span class="proto-tag proto-${span.protocol}">${span.protocol}</span> 
              <span style="color:var(--text-muted);">${escapeHtml(span.name)}</span>
            </span>
            <span style="font-family:var(--font-mono); font-size:0.8rem;">${span.duration_ms.toFixed(2)} ms</span>
          </div>
          ${span.sql_query ? `<div style="font-family:var(--font-mono); font-size:0.75rem; color:var(--accent-amber); padding:4px 0;">SQL: ${escapeHtml(span.sql_query)}</div>` : ''}
          <div class="waterfall-bar-track">
            <div class="waterfall-bar-fill" style="width: ${widthPct}%; ${isError ? 'background:var(--accent-rose);' : ''}"></div>
          </div>
        </div>
      `;
    });

    elements.waterfallContainer.innerHTML = html;
  }

  function updateMetricsUI(metrics) {
    if (!metrics) return;

    if (elements.statSpans) elements.statSpans.textContent = metrics.total_spans_captured || 0;
    if (elements.statTraces) elements.statTraces.textContent = metrics.active_traces || 0;
    if (elements.statLatency) elements.statLatency.textContent = `${(metrics.average_latency_ms || 0).toFixed(2)} ms`;
    if (elements.statErrors) elements.statErrors.textContent = metrics.total_errors || 0;

    if (elements.tracesCountBadge) {
      elements.tracesCountBadge.textContent = `${metrics.total_spans_captured || 0} spans`;
    }

    if (elements.statErrorFooter && metrics.total_spans_captured > 0) {
      const rate = ((metrics.total_errors / metrics.total_spans_captured) * 100).toFixed(1);
      elements.statErrorFooter.textContent = `${rate}% Error Rate`;
      elements.statErrorFooter.style.color = metrics.total_errors > 0 ? 'var(--accent-rose)' : 'var(--accent-emerald)';
    }

    if (metrics.topology_edges) {
      renderTopology(metrics.topology_edges, metrics.active_services);
    }
  }

  function renderTopology(edges, services) {
    if (!elements.topologyContainer) return;

    if (!services || services.length === 0) {
      elements.topologyContainer.innerHTML = '<div style="color:var(--text-muted); padding:20px;">No microservice topology dependencies discovered yet.</div>';
      return;
    }

    let html = '';
    services.forEach(service => {
      const serviceEdges = (edges || []).filter(e => e.source_service === service);

      html += `
        <div class="topology-card">
          <div class="topology-name">📍 ${escapeHtml(service)}</div>
          <div class="topology-meta">${serviceEdges.length} outbound dependency links</div>
          ${serviceEdges.map(e => `
            <div style="font-size:0.8rem; color:var(--accent-cyan); margin-top:4px;">
              ➔ ${escapeHtml(e.target_service)} (${e.protocol}) - ${e.avg_latency_ms.toFixed(1)}ms
            </div>
          `).join('')}
        </div>
      `;
    });

    elements.topologyContainer.innerHTML = html;
  }

  function filterTraces(query) {
    if (!query) {
      renderTracesFeed();
      return;
    }
    const q = query.toLowerCase();
    const filtered = state.traces.filter(t => 
      t.trace_id.toLowerCase().includes(q) || 
      t.correlation_id.toLowerCase().includes(q) ||
      (t.root_span && t.root_span.service_name.toLowerCase().includes(q))
    );

    state.traces = filtered;
    renderTracesFeed();
  }

  function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  // Boot Application
  document.addEventListener('DOMContentLoaded', init);
})();
