document.addEventListener('DOMContentLoaded', () => {
    setInterval(fetchMonitoringData, 6000); // Adjust interval to 6 seconds
    fetchMonitoringData(); // Initial fetch
});

function fetchMonitoringData() {
    console.log('Fetching new data from /monitor');
    fetch('/monitor', { cache: 'no-store' })
        .then(response => response.json())
        .then(data => {
            console.log('Fetched data:', data); // Log the fetched data for debugging
            updateTable(data);
        })
        .catch(error => {
            console.error('Error fetching data:', error);
        });
}

function updateTable(data) {
    const tbody = document.querySelector('#monitorTable tbody');
    if (!tbody) {
        console.error('Table body element not found');
        return;
    }
    
    console.log('Clearing existing table rows');
    // Clear existing rows
    while (tbody.firstChild) {
        tbody.removeChild(tbody.firstChild);
    }

    if (!Array.isArray(data)) {
        console.error('Data is not an array:', data);
        return;
    }

    console.log('Updating table with new data');
    data.forEach(segment => {
        console.log(`Segment URL: ${segment.url}, IsDelayed: ${segment.isDelayed}`); // Log the segment details for debugging

        const row = document.createElement('tr');
        if (segment.isDelayed) {
            row.classList.add('delayed');
        }

        row.innerHTML = `
            <td>${segment.url}</td>
            <td>${segment.duration}</td>
            <td>${segment.load_time}</td>
            <td>${segment.isDelayed ? 'Delayed' : 'OK'}</td>
        `;
        tbody.appendChild(row);
    });
}
