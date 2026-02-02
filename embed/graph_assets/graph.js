window.tasktreeGraphServerAssetsLoaded = true;

// Persistent state tracking for expanded items (survives DOM rebuilds)
const expandedFeatures = new Set();
const expandedTasks = new Set();
const infoExpandedFeatures = new Set();

// Feature list toggle functionality
function toggleFeatureTasks(headerElement) {
  const featureGroup = headerElement.parentElement;
  const featureName = featureGroup.getAttribute('data-feature');
  const isExpanding = !featureGroup.classList.contains('expanded');

  if (isExpanding) {
    // Collapse all other feature groups first
    document.querySelectorAll('.feature-group').forEach(group => {
      group.classList.remove('expanded');
    });
    expandedFeatures.clear();

    featureGroup.classList.add('expanded');
    if (featureName) {
      expandedFeatures.add(featureName);
    }
  } else {
    // Collapsing - just remove this feature
    featureGroup.classList.remove('expanded');
    if (featureName) {
      expandedFeatures.delete(featureName);
    }
  }
}

function toggleFeatureInfo(event, toggleElement) {
  event.stopPropagation();
  const headerElement = toggleElement.closest('.feature-header');
  const featureGroup = headerElement.closest('.feature-group');
  const featureName = featureGroup ? featureGroup.getAttribute('data-feature') : null;
  const isExpanding = !headerElement.classList.contains('info-expanded');

  if (isExpanding) {
    headerElement.classList.add('info-expanded');
    toggleElement.classList.add('active');
    toggleElement.textContent = 'Hide Info';
    if (featureName) {
      infoExpandedFeatures.add(featureName);
    }
  } else {
    headerElement.classList.remove('info-expanded');
    toggleElement.classList.remove('active');
    toggleElement.textContent = 'Info';
    if (featureName) {
      infoExpandedFeatures.delete(featureName);
    }
  }
}

// Task list toggle functionality
function toggleTaskDetails(headerElement) {
  const taskItem = headerElement.parentElement;
  const expandIcon = headerElement.querySelector('.task-expand-icon');
  const taskName = taskItem.getAttribute('data-task-name');

  // Check if this task is currently expanded
  const isExpanding = !taskItem.classList.contains('selected');

  // Close all other expanded tasks first
  document.querySelectorAll('.task-expand-icon').forEach(icon => {
    icon.classList.remove('expanded');
  });
  document.querySelectorAll('.task-item').forEach(item => {
    item.removeAttribute('data-expanded');
    item.classList.remove('selected');
  });
  expandedTasks.clear();

  if (isExpanding) {
    // Now expand this task
    expandIcon.classList.add('expanded');
    taskItem.setAttribute('data-expanded', 'true');
    taskItem.classList.add('selected');
    if (taskName) {
      expandedTasks.add(taskName);
    }
  }

  // Bidirectional sync: Highlight and center node in graph if not already selected
  if (typeof node !== 'undefined' && node) {
    const d = node.data().find(n => n.name === taskName);
    if (d && selectedNodeId !== d.id) {
      centerNode(d);
    }
  }
}

function centerNode(d) {
  selectedNodeId = d.id;
  if (node) {
    node.attr('stroke', getNodeStroke)
        .attr('stroke-width', getNodeStrokeWidth);
  }

  const scale = d3.zoomTransform(svg.node()).k;
  const centerX = (WIDTH + SIDEBAR_WIDTH) / 2;
  const centerY = HEIGHT / 2;

  const transform = d3.zoomIdentity
    .translate(centerX, centerY)
    .scale(scale)
    .translate(-d.x, -d.y);

  svg.transition()
    .duration(750)
    .call(zoom.transform, transform);
}

function handleNodeClick(event, d) {
  centerNode(d);

  // 1. Expand feature and task in sidebar
  const featureGroup = document.querySelector(`.feature-group[data-feature="${d.feature_name}"]`);
  if (featureGroup) {
    const featureHeader = featureGroup.querySelector('.feature-header');
    if (!featureGroup.classList.contains('expanded')) {
      toggleFeatureTasks(featureHeader);
    }

    const taskItem = featureGroup.querySelector(`.task-item[data-task-name="${d.name}"]`);
    if (taskItem) {
      const taskHeader = taskItem.querySelector('.task-header');
      // Remove selected class first so toggleTaskDetails can expand it
      taskItem.classList.remove('selected');
      toggleTaskDetails(taskHeader);
      taskItem.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  }
}

// Helper to format duration in seconds as mm:ss or hh:mm:ss
function formatDuration(seconds) {
  if (seconds === null || seconds === undefined) return '';
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  const mm = m.toString().padStart(2, '0');
  const ss = s.toString().padStart(2, '0');
  if (h > 0) {
    return `${h}:${mm}:${ss}`;
  }
  return `${mm}:${ss}`;
}

// Configuration
const API_ENDPOINT = '/api/graph';
const TASKS_ENDPOINT = '/api/tasks';
const FEATURES_ENDPOINT = '/api/features';
let WIDTH = window.innerWidth;
let HEIGHT = window.innerHeight;
let SIDEBAR_WIDTH = WIDTH * 0.24;
let isDragging = false;
let selectedNodeId = null;

// Status colors
const STATUS_COLORS = {
  pending: '#2e3c62',
  in_progress: '#fff000',
  completed: '#22d3ee',
  blocked: '#f43f5e',
};

// Create SVG
const svg = d3.select('#graph')
  .append('svg')
  .attr('width', WIDTH)
  .attr('height', HEIGHT);

// Define arrowhead marker for directed edges
const defs = svg.append('defs');

defs.append('marker')
  .attr('id', 'arrowhead')
  .attr('viewBox', '0 -5 10 10')
  .attr('refX', 20)
  .attr('refY', 0)
  .attr('markerWidth', 4.5)
  .attr('markerHeight', 4.5)
  .attr('orient', 'auto')
  .append('path')
  .attr('d', 'M0,-5L10,0L0,5')
  .attr('fill', '#2e3c62');

// Create container for zoom/pan
const container = svg.append('g');

// Add zoom behavior
const zoom = d3.zoom()
  .scaleExtent([0.1, 4])
  .on('zoom', (event) => {
    container.attr('transform', event.transform);
  });

svg.call(zoom);

// Initialize force simulation (aligned with Observable disjoint-force pattern)
const simulation = d3.forceSimulation()
  .force('link', d3.forceLink().id(d => d.id).distance(150).strength(0.2))
  .force('forceX', d3.forceX((WIDTH + SIDEBAR_WIDTH) / 2).strength(0.15))
  .force('forceY', d3.forceY(HEIGHT / 2).strength(0.15))
  .force('charge', d3.forceManyBody().strength(-1000));

// Graph elements
let linkGroup = container.append('g').attr('class', 'links');
let nodeGroup = container.append('g').attr('class', 'nodes');
let labelGroup = container.append('g').attr('class', 'labels');

let link, node, label;


function getNodeColor(d) {
  if (d.is_available) return '#a855f7';
  return STATUS_COLORS[d.status] || '#999';
}

function getNodeRadius(d) {
  return 4 + (d.priority / 10) * 11;
}

function getNodeStroke(d) {
  return d.id === selectedNodeId ? '#ffffff' : 'none';
}

function getNodeStrokeWidth(d) {
  return d.id === selectedNodeId ? 3 : 0;
}

function seedNodePositions(nodes, positionCache) {
  const centerX = (WIDTH + SIDEBAR_WIDTH) / 2;
  const centerY = HEIGHT / 2;

  nodes.forEach(node => {
    if (positionCache.has(node.id)) return;

    const angle = Math.random() * Math.PI * 2;
    const radius = 40 + Math.random() * 40;

    if (!Number.isFinite(node.x)) {
      node.x = centerX + Math.cos(angle) * radius;
    }
    if (!Number.isFinite(node.y)) {
      node.y = centerY + Math.sin(angle) * radius;
    }

    if (!Number.isFinite(node.vx)) node.vx = 0;
    if (!Number.isFinite(node.vy)) node.vy = 0;
  });
}

function updateGraph(graphData) {
  // Update loading state
  d3.select('#loading').style('display', 'none');

  // Store current positions BEFORE data update
  const positionCache = new Map();
  if (node) {
    node.each(d => {
      positionCache.set(d.id, {
        x: d.x,
        y: d.y,
        vx: d.vx || 0,
        vy: d.vy || 0,
        fx: d.fx,
        fy: d.fy,
      });
    });
  }

  // Convert edges from {from, to} to {source, target}
  const links = graphData.edges.map(e => ({
    source: e.from,
    target: e.to,
  }));

  const nodes = graphData.nodes;

  seedNodePositions(nodes, positionCache);

  // Store previous counts for change detection
  const previousNodeCount = positionCache.size;
  const previousLinkCount = link ? link.size() : 0;

  // Update links
  link = linkGroup.selectAll('.link')
    .data(links, d => {
      const source = typeof d.source === 'object' ? d.source.id : d.source;
      const target = typeof d.target === 'object' ? d.target.id : d.target;
      return `${source}-${target}`;
    })
    .join('path')
    .attr('class', 'link');

  // Update nodes
  node = nodeGroup.selectAll('.node')
    .data(nodes, d => d.id)
    .join('circle')
    .attr('class', 'node')
    .call(d3.drag()
      .on('start', dragstarted)
      .on('drag', dragged)
      .on('end', dragended))
    .on('click', handleNodeClick);

  // Restore positions for existing nodes AFTER data join
  node.each(d => {
    const cached = positionCache.get(d.id);
    if (cached) {
      Object.assign(d, cached);
    }
  });

  node.attr('r', getNodeRadius)
    .attr('fill', getNodeColor)
    .attr('stroke', getNodeStroke)
    .attr('stroke-width', getNodeStrokeWidth)
    .attr('data-status', d => d.status)
    .style('--base-radius', d => getNodeRadius(d) + 'px')
    .attr('filter', d => {
      if (d.is_available) return 'url(#glow-available)';
      if (d.status === 'in_progress') return 'url(#glow-in-progress)';
      if (d.status === 'pending') return null;
      return 'url(#glow-status)';
    });

  // Update labels
  label = labelGroup.selectAll('.node-label')
    .data(nodes, d => d.id)
    .join('text')
    .attr('class', 'node-label')
    .attr('dy', 4)
    .text(d => d.name);

  // Detect structural changes
  const structureChanged =
    nodes.length !== previousNodeCount ||
    links.length !== previousLinkCount;

  // Update simulation
  simulation.nodes(nodes).on('tick', ticked);
  simulation.force('link').links(links);

  // Reheat simulation if structure changed or to resolve new collisions/radii
  if (structureChanged) {
    simulation.alpha(0.3).restart();
  } else {
    // Subtle reheat to update link ends if radii changed and allow re-centering
    simulation.alpha(0.1).restart();
  }
}

function ticked() {
  link.attr('d', d => {
    const sourceX = d.source.x;
    const sourceY = d.source.y;
    const targetX = d.target.x;
    const targetY = d.target.y;

    // Calculate the direction vector
    const dx = targetX - sourceX;
    const dy = targetY - sourceY;
    const dist = Math.sqrt(dx * dx + dy * dy);

    if (dist === 0) return `M${sourceX},${sourceY}L${targetX},${targetY}`;

    // Adjust the end point to stop at the target node's edge
    const targetRadius = getNodeRadius(d.target);
    const offsetX = (dx / dist) * targetRadius;
    const offsetY = (dy / dist) * targetRadius;

    return `M${sourceX},${sourceY}L${targetX - offsetX},${targetY - offsetY}`;
  });

  node.attr('cx', d => d.x)
    .attr('cy', d => d.y);

  label.attr('x', d => d.x + getNodeRadius(d) * 0.7)
    .attr('y', d => d.y - getNodeRadius(d) * 0.7);
}

function dragstarted(event, d) {
  isDragging = true;
  if (!event.active) simulation.alphaTarget(0.3).restart();
  d.fx = d.x;
  d.fy = d.y;
}

function dragged(event, d) {
  d.fx = event.x;
  d.fy = event.y;
}

function dragended(event, d) {
  isDragging = false;
  if (!event.active) simulation.alphaTarget(0);
  d.fx = null;
  d.fy = null;
}

function showError(message) {
  d3.select('#loading').style('display', 'none');

  const errorDiv = d3.select('body')
    .append('div')
    .attr('class', 'error-message')
    .text(message);

  setTimeout(() => errorDiv.remove(), 5000);
}

async function fetchGraph() {
  try {
    const response = await fetch(API_ENDPOINT);

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    const data = await response.json();
    updateGraph(data);
  } catch (error) {
    console.error('Error fetching graph:', error);
    showError(`Failed to fetch graph: ${error.message}`);
  }
}

async function fetchTasks() {
  try {
    const [tasksResponse, featuresResponse] = await Promise.all([
      fetch(TASKS_ENDPOINT),
      fetch(FEATURES_ENDPOINT)
    ]);

    if (!tasksResponse.ok) {
      throw new Error(`Tasks HTTP ${tasksResponse.status}: ${tasksResponse.statusText}`);
    }
    if (!featuresResponse.ok) {
      throw new Error(`Features HTTP ${featuresResponse.status}: ${featuresResponse.statusText}`);
    }

    const tasks = await tasksResponse.json();
    const features = await featuresResponse.json();
    updateTaskList(tasks, features);
  } catch (error) {
    console.error('Error fetching tasks/features:', error);
  }
}

function updateTaskList(tasks, features) {
  const taskListDiv = document.querySelector('.task-list');

  if (!tasks || tasks.length === 0) {
    taskListDiv.innerHTML = '<div style="padding: 12px; color: #7c3aed; font-size: 12px;">No tasks available</div>';
    return;
  }

  // Clear any placeholder text (e.g., "Loading tasks...") when tasks arrive
  const placeholder = taskListDiv.querySelector(':scope > .task-list-placeholder');
  if (placeholder) {
    placeholder.remove();
  }

  // Map features by name for easy lookup
  const featuresByName = new Map();
  if (features) {
    features.forEach(f => featuresByName.set(f.name, f));
  }

  // Status colors
  const statusColors = {
    pending: '#2e3c62',
    in_progress: '#fff000',
    completed: '#22d3ee',
    blocked: '#f43f5e',
  };

  // Group tasks by feature
  const tasksByFeature = new Map();
  tasks.forEach(task => {
    if (!tasksByFeature.has(task.feature_name)) {
      tasksByFeature.set(task.feature_name, []);
    }
    tasksByFeature.get(task.feature_name).push(task);
  });

  const sortedFeatures = Array.from(tasksByFeature.keys()).sort((a, b) =>
    a.localeCompare(b)
  );

  // Track which feature groups should exist
  const currentFeatures = new Set(sortedFeatures);

  // Remove feature groups that no longer exist
  taskListDiv.querySelectorAll('.feature-group').forEach(group => {
    const featureName = group.getAttribute('data-feature');
    if (!currentFeatures.has(featureName)) {
      group.remove();
    }
  });

  // Update or create feature groups
  sortedFeatures.forEach(featureName => {
    const featureTasks = tasksByFeature.get(featureName) || [];
    const isExpanded = expandedFeatures.has(featureName);
    const featureColor = featureTasks.length > 0 && featureTasks[0].feature_color ? featureTasks[0].feature_color : '#cccccc';

    let featureGroup = taskListDiv.querySelector(`.feature-group[data-feature="${featureName}"]`);

    // Create feature group if it doesn't exist
    if (!featureGroup) {
      featureGroup = document.createElement('div');
      featureGroup.className = 'feature-group';
      featureGroup.setAttribute('data-feature', featureName);
      taskListDiv.appendChild(featureGroup);
    }

    // Update expanded class (preserves animation state if already expanded)
    if (isExpanded) {
      featureGroup.classList.add('expanded');
    } else {
      featureGroup.classList.remove('expanded');
    }

    // Update highlight status
    const inProgress = featureTasks.some(t => t.status === 'in_progress');
    const available = featureTasks.some(t => t.status === 'pending' && t.is_available);
    featureGroup.removeAttribute('data-highlight-status');
    if (inProgress) {
      featureGroup.setAttribute('data-highlight-status', 'in_progress');
    } else if (available) {
      featureGroup.setAttribute('data-highlight-status', 'available');
    }

    // Get or create feature header
    let featureHeader = featureGroup.querySelector('.feature-header');
    if (!featureHeader) {
      featureHeader = document.createElement('div');
      featureHeader.className = 'feature-header';
      featureHeader.onclick = function() { toggleFeatureTasks(this); };
      featureGroup.appendChild(featureHeader);
    }

    // Update info-expanded class
    const isInfoExpanded = infoExpandedFeatures.has(featureName);
    if (isInfoExpanded) {
      featureHeader.classList.add('info-expanded');
    } else {
      featureHeader.classList.remove('info-expanded');
    }

    // Get or create main info section
    let mainInfo = featureHeader.querySelector('.feature-main-info');
    if (!mainInfo) {
      mainInfo = document.createElement('div');
      mainInfo.className = 'feature-main-info';
      featureHeader.appendChild(mainInfo);
    }

    // Update feature header content
    const completedTasks = featureTasks.filter(t => t.status === 'completed').length;
    const totalTasks = featureTasks.length;
    const allCompleted = completedTasks === totalTasks && totalTasks > 0;
    const countStyle = allCompleted ? 'color: #22d3ee; font-weight: bold;' : '';

    mainInfo.innerHTML =
      '<span class="feature-chevron">▶</span>' +
      `<span class="feature-name" title="${featureName}">${featureName}</span>` +
      `<span class="feature-count" style="${countStyle}">${completedTasks} / ${totalTasks}</span>` +
      `<span class="feature-info-toggle ${isInfoExpanded ? 'active' : ''}" onclick="toggleFeatureInfo(event, this)">${isInfoExpanded ? 'Hide Info' : 'Info'}</span>`;

    // Update feature meta info
    let featureMeta = featureHeader.querySelector('.feature-meta-info');
    const featureData = featuresByName.get(featureName);
    if (featureData) {
      if (!featureMeta) {
        featureMeta = document.createElement('div');
        featureMeta.className = 'feature-meta-info';
        featureHeader.appendChild(featureMeta);
      }
      featureMeta.innerHTML =
        '<span class="feature-meta-label">Description</span>' +
        `<div class="feature-meta-value">${featureData.description ? marked.parse(featureData.description) : 'No description'}</div>` +
        '<span class="feature-meta-label">Specification</span>' +
        `<div class="feature-meta-value">${featureData.specification ? marked.parse(featureData.specification) : 'No specification'}</div>`;
    } else if (featureMeta) {
      featureMeta.remove();
    }

    // Get or create feature tasks container
    let featureTasksContainer = featureGroup.querySelector('.feature-tasks');
    if (!featureTasksContainer) {
      featureTasksContainer = document.createElement('div');
      featureTasksContainer.className = 'feature-tasks';
      featureGroup.appendChild(featureTasksContainer);
    }

    let tasksInner = featureTasksContainer.querySelector('.feature-tasks-inner');
    if (!tasksInner) {
      tasksInner = document.createElement('div');
      tasksInner.className = 'feature-tasks-inner';
      featureTasksContainer.appendChild(tasksInner);
    }

    // Track which tasks should exist
    const currentTaskNames = new Set(featureTasks.map(t => t.name));

    // Remove tasks that no longer exist
    tasksInner.querySelectorAll('.task-item').forEach(item => {
      const taskName = item.getAttribute('data-task-name');
      if (!currentTaskNames.has(taskName)) {
        item.remove();
      }
    });

    // Update or create task items
    featureTasks.forEach(task => {
      let statusColor = statusColors[task.status] || '#999';
      if (task.status === 'pending' && task.is_available) {
        statusColor = '#a855f7';
      }

      const shouldExpand = expandedTasks.has(task.name);
      const isSelected = task.id === selectedNodeId || shouldExpand;

      let taskItem = tasksInner.querySelector(`.task-item[data-task-name="${task.name}"]`);

      // Create task item if it doesn't exist
      if (!taskItem) {
        taskItem = document.createElement('div');
        taskItem.className = 'task-item';
        taskItem.setAttribute('data-task-name', task.name);
        taskItem.setAttribute('data-feature', task.feature_name);
        tasksInner.appendChild(taskItem);
      }

      // Update task attributes
      taskItem.setAttribute('data-status', task.status);
      taskItem.style.background = statusColor + '0d';

      if (shouldExpand) {
        taskItem.setAttribute('data-expanded', 'true');
        taskItem.classList.add('selected');
      } else {
        taskItem.removeAttribute('data-expanded');
        taskItem.classList.remove('selected');
      }

      // Get or create task header
      let taskHeader = taskItem.querySelector('.task-header');
      if (!taskHeader) {
        taskHeader = document.createElement('div');
        taskHeader.className = 'task-header';
        taskHeader.onclick = function() { toggleTaskDetails(this); };
        taskItem.appendChild(taskHeader);
      }

      // Calculate duration for header
      let durationHeaderHtml = '';
      if (task.started_at && task.completed_at) {
        const start = new Date(task.started_at);
        const end = new Date(task.completed_at);
        const seconds = Math.round((end - start) / 1000);
        durationHeaderHtml = `<span class="task-duration">${formatDuration(seconds)}</span>`;
      }

      taskHeader.innerHTML =
        `<span class="task-status-dot" style="background: ${statusColor};"></span>` +
        `<span class="task-name" title="${task.name}">${task.name}</span>` +
        durationHeaderHtml +
        `<span class="task-expand-icon ${shouldExpand ? 'expanded' : ''}">▶</span>`;

      // Get or create task details
      let taskDetails = taskItem.querySelector('.task-details');
      if (!taskDetails) {
        taskDetails = document.createElement('div');
        taskDetails.className = 'task-details';
        taskItem.appendChild(taskDetails);
      }

      let detailsInner = taskDetails.querySelector('.task-details-inner');
      if (!detailsInner) {
        detailsInner = document.createElement('div');
        detailsInner.className = 'task-details-inner';
        taskDetails.appendChild(detailsInner);
      }

      // Build details HTML
      let detailsHtml = `<div class="task-details-row"><span class="task-details-label">Status:</span> ${task.status}</div>` +
        `<div class="task-details-row"><span class="task-details-label">Priority:</span> ${task.priority}</div>` +
        `<div class="task-details-row"><span class="task-details-label">Description:</span>` +
        `<div class="task-details-value">${task.description ? marked.parse(task.description) : 'None'}</div></div>`;

      if (task.specification && task.specification !== task.description) {
        detailsHtml += `<div class="task-details-row"><span class="task-details-label">Spec:</span>` +
          `<div class="task-details-value">${marked.parse(task.specification)}</div></div>`;
      }

      if (task.status === 'completed' && task.completion_summary) {
        detailsHtml += `<div class="task-details-row"><span class="task-details-label">Summary:</span>` +
          `<div class="task-details-value task-completion-summary">${marked.parse(task.completion_summary)}</div></div>`;
      }

      detailsHtml += `<div class="task-details-row"><span class="task-details-label">Created:</span> ${task.created_at || 'None'}</div>`;

      if (task.started_at) {
        detailsHtml += `<div class="task-details-row"><span class="task-details-label">Started:</span> ${task.started_at}</div>`;
      }

      if (task.completed_at) {
        detailsHtml += `<div class="task-details-row"><span class="task-details-label">Completed:</span> ${task.completed_at}</div>`;
        if (task.started_at) {
          const start = new Date(task.started_at);
          const end = new Date(task.completed_at);
          const seconds = Math.round((end - start) / 1000);
          detailsHtml += `<div class="task-details-row"><span class="task-details-label">Duration:</span> ${formatDuration(seconds)}</div>`;
        }
      }

      detailsInner.innerHTML = detailsHtml;
    });
  });
}

// Initial fetch
fetchGraph();
fetchTasks();

// Auto-refresh every 3 seconds
setInterval(() => {
  fetchGraph();
  fetchTasks();
}, 3000);

// Handle window resize
window.addEventListener('resize', () => {
  WIDTH = window.innerWidth;
  HEIGHT = window.innerHeight;
  SIDEBAR_WIDTH = WIDTH * 0.24;
  const newWidth = WIDTH;
  const newHeight = HEIGHT;

  svg.attr('width', newWidth).attr('height', newHeight);

  simulation.force('forceX', d3.forceX((newWidth + SIDEBAR_WIDTH) / 2).strength(0.1));
  simulation.force('forceY', d3.forceY(newHeight / 2).strength(0.1));
  simulation.alpha(0.3).restart();
});

// Initial render for markdown in the static HTML
document.querySelectorAll('.task-details-value').forEach(el => {
  el.innerHTML = marked.parse(el.textContent);
});
