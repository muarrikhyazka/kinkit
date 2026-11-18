import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js'
import { Pie } from 'react-chartjs-2'
import ChartDataLabels from 'chartjs-plugin-datalabels'
import { categoryColor, chartTheme, readableOn } from './categoryColor'
import { formatRupiah } from './api'

ChartJS.register(ArcElement, Tooltip, Legend, ChartDataLabels)

/** Chart.js pie chart - hoverable slices, percentage labels, no collisions. */
export default function PieChart({ data }) {
  const total = data.reduce((s, d) => s + d.total, 0)
  if (total <= 0) return <div className="muted">No data yet.</div>

  const { text: textColor, card } = chartTheme()
  const colors = data.map(d => categoryColor(d.category_name))

  const chartData = {
    labels: data.map(d => d.category_name),
    datasets: [{
      data: data.map(d => d.total),
      backgroundColor: colors,
      borderColor: card,
      borderWidth: 2,
    }],
  }

  const options = {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { position: 'right', labels: { color: textColor, boxWidth: 10, font: { size: 11 } } },
      tooltip: {
        callbacks: {
          label: (ctx) => `${ctx.label}: ${formatRupiah(ctx.raw)} (${Math.round((ctx.raw / total) * 100)}%)`,
        },
      },
      datalabels: {
        display: (ctx) => ctx.dataset.data[ctx.dataIndex] / total >= 0.06,
        color: (ctx) => readableOn(colors[ctx.dataIndex]),
        font: { size: 11, weight: 700 },
        formatter: (v) => `${Math.round((v / total) * 100)}%`,
      },
    },
  }

  return <div style={{ height: 260 }}><Pie data={chartData} options={options} /></div>
}
