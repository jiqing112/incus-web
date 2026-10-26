/** 数字格式化工具：等宽显示、字节/速率人性化。 */

export function fmtBytes(n: number, digits = 1): string {
	if (!Number.isFinite(n)) return '—';
	const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
	let v = Math.abs(n);
	let i = 0;
	while (v >= 1024 && i < units.length - 1) {
		v /= 1024;
		i++;
	}
	const d = i === 0 ? 0 : v >= 100 ? 0 : digits;
	return `${v.toFixed(d)} ${units[i]}`;
}

export function fmtRate(bytesPerSec: number, digits = 1): string {
	if (!Number.isFinite(bytesPerSec)) return '—';
	return `${fmtBytes(bytesPerSec, digits)}/s`;
}

export function fmtNum(n: number): string {
	if (!Number.isFinite(n)) return '—';
	return new Intl.NumberFormat('zh-CN').format(n);
}

export function fmtCompact(n: number): string {
	if (!Number.isFinite(n)) return '—';
	return new Intl.NumberFormat('zh-CN', { notation: 'compact', maximumFractionDigits: 1 }).format(n);
}

export function fmtTime(ts: number | Date): string {
	const d = ts instanceof Date ? ts : new Date(ts);
	return d.toLocaleTimeString('zh-CN', { hour12: false });
}

export function fmtDateTime(ts: number | Date | string | null | undefined): string {
	if (!ts) return '—';
	const d = ts instanceof Date ? ts : new Date(ts);
	if (Number.isNaN(d.getTime())) return String(ts);
	return d.toLocaleString('zh-CN', { hour12: false });
}

export function relativeTime(ts: number | Date | string | null | undefined): string {
	if (!ts) return '—';
	const d = ts instanceof Date ? ts : new Date(ts);
	if (Number.isNaN(d.getTime())) return String(ts);
	let diff = Math.floor((Date.now() - d.getTime()) / 1000);
	const future = diff < 0;
	diff = Math.abs(diff);
	const label = (v: number, unit: string) => (future ? `${v} ${unit}后` : `${v} ${unit}前`);
	if (diff < 10) return future ? '即将' : '刚刚';
	if (diff < 60) return label(diff, '秒');
	if (diff < 3600) return label(Math.floor(diff / 60), '分钟');
	if (diff < 86400) return label(Math.floor(diff / 3600), '小时');
	return label(Math.floor(diff / 86400), '天');
}
