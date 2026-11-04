export const isoWeek = (date: Date): string => {
    const thursday = new Date(Date.UTC(date.getFullYear(), date.getMonth(), date.getDate()));
    thursday.setUTCDate(thursday.getUTCDate() + 4 - (thursday.getUTCDay() || 7));
    const year = thursday.getUTCFullYear();
    const first = Date.UTC(year, 0, 1);
    const week = Math.ceil(((thursday.getTime() - first) / 86400000 + 1) / 7);
    return `CW${String(week).padStart(2, '0')}`;
};
