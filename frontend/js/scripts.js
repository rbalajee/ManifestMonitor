let loadTimeChart;
let segmentLoadTimes = new Map();
let sessionID = generateSessionID();

function generateSessionID() {
    return '_' + Math.random().toString(36).substr(2, 9);
}

async function startMonitoring() {
    const url = document.getElementById('manifestUrl').value.trim();
    if (validateUrl(url)) {
        try {
            const response = await fetch('/startMonitoring', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({ url, id: sessionID })
            });
            if (response.ok) {
                console.log('Monitoring started');
                fetchSegments();
                setInterval(fetchSegments, 6000);
            } else {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
        } catch (error) {
            console.error('Error starting monitoring:', error);
        }
    } else {
        console.error('Invalid URL');
    }
}

function validateUrl(url) {
    const urlPattern = new RegExp('^(https?:\\/\\/)?' + // protocol
        '((([a-z\\d]([a-z\\d-]*[a-z\\d])*)\\.?)+[a-z]{2,}|' + // domain name
        '((\\d{1,3}\\.){3}\\d{1,3}))' + // OR ip (v4) address
        '(\\:\\d+)?(\\/[-a-z\\d%_.~+]*)*' + // port and path
        '(\\?[;&a-z\\d%_.~+=-]*)?' + // query string
        '(\\#[-a-z\\d_]*)?$', 'i'); // fragment locator
    return !!urlPattern.test(url);
}

async function fetchSegments() {
    try {
        const response = await fetch(`/monitor?id=${sessionID}`);
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const segments = await response.json();
        if (!Array.isArray(segments)) {
            throw new Error('Response is not an array');
        }

        const segmentTableBody = document.getElementById('segmentTable').getElementsByTagName('tbody')[0];
        segmentTableBody.innerHTML = '';
        segments.forEach(segment => {
            const chunkNumber = extractChunkNumber(segment.URL);
            segmentLoadTimes.set(chunkNumber, segment.LoadTime);

            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${escapeHtml(String(segment.URL))}</td>
                <td>${escapeHtml(String(segment.Duration))}</td>
                <td>${escapeHtml(String(segment.LoadTime))}</td>
                <td>${escapeHtml(String(segment.IsDelayed))}</td>
            `;
            segmentTableBody.appendChild(row);
        });

        updateChart(segmentLoadTimes);
    } catch (error) {
        console.error('Error fetching segments:', error);
    }
}

function escapeHtml(unsafe) {
    if (typeof unsafe !== 'string') {
        return unsafe;
    }
    return unsafe
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

function extractChunkNumber(url) {
    const match = url.match(/(\d+)\.ts/);
    return match ? match[1] : url;
}

function updateChart(segmentLoadTimes) {
    const ctx = document.getElementById('loadTimeChart').getContext('2d');
    const chunkNumbers = Array.from(segmentLoadTimes.keys());
    const loadTimes = Array.from(segmentLoadTimes.values());

    if (!loadTimeChart) {
        loadTimeChart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: chunkNumbers,
                datasets: [{
                    label: 'Load Time (s)',
                    data: loadTimes,
                    backgroundColor: 'rgba(0, 173, 181, 0.2)', // Soft teal for background
                    borderColor: '#00adb5', // Vibrant teal for line
                    borderWidth: 2,
                    pointBackgroundColor: '#00adb5', // Vibrant teal for points
                    pointBorderColor: '#2a2a2a', // Dark grey for point borders
                    pointHoverBackgroundColor: '#00d8d6', // Lighter teal for hovered points
                    pointHoverBorderColor: '#2a2a2a' // Dark grey for hovered point borders
                }]
            },
            options: {
                scales: {
                    x: {
                        title: {
                            display: true,
                            text: 'TS Segment (#)',
                            color: '#eaeaea'
                        },
                        ticks: {
                            color: '#eaeaea'
                        }
                    },
                    y: {
                        title: {
                            display: true,
                            text: 'Load Time (s)',
                            color: '#eaeaea'
                        },
                        ticks: {
                            color: '#eaeaea'
                        },
                        beginAtZero: true
                    }
                },
                plugins: {
                    legend: {
                        labels: {
                            color: '#eaeaea'
                        }
                    }
                }
            }
        });
    } else {
        loadTimeChart.data.labels = chunkNumbers;
        loadTimeChart.data.datasets[0].data = loadTimes;
        loadTimeChart.update();
    }
}

window.addEventListener('beforeunload', function (event) {
    navigator.sendBeacon('/stopMonitoring', JSON.stringify({ id: sessionID }));
});
