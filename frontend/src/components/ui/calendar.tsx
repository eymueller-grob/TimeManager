import {DayPicker} from 'react-day-picker';
import {cn} from '@/lib/utils';

const calendarClassNames = {
    months: 'flex flex-col gap-2',
    month: 'flex flex-col gap-4',
    month_caption: 'relative flex h-9 items-center justify-center',
    caption_label: 'text-sm font-medium',
    nav: 'flex items-center',
    button_previous: 'absolute left-1 inline-flex size-8 items-center justify-center rounded-lg border border-transparent bg-transparent text-sm hover:bg-muted',
    button_next: 'absolute right-1 inline-flex size-8 items-center justify-center rounded-lg border border-transparent bg-transparent text-sm hover:bg-muted',
    chevron: 'size-4 fill-none stroke-current',
    month_grid: 'w-full border-collapse',
    weekdays: 'flex',
    weekday: 'w-9 rounded-md text-center text-xs font-normal text-muted-foreground',
    week: 'mt-1 flex w-full',
    week_number_header: 'w-9 text-center text-xs font-normal text-muted-foreground',
    week_number: 'flex size-9 items-center justify-center text-xs font-normal text-muted-foreground',
    day: 'relative size-9 rounded-lg p-0 text-center text-sm',
    day_button: 'inline-flex size-9 items-center justify-center rounded-lg border border-transparent bg-transparent p-0 font-normal hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
    selected: 'text-primary-foreground',
    today: 'font-semibold text-primary',
    outside: 'text-muted-foreground opacity-50',
    disabled: 'pointer-events-none opacity-40',
    hidden: 'invisible',
} as const;

const dateKey = (date: Date) => {
    const pad = (value: number) => String(value).padStart(2, '0');
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
};

export function Calendar({className, classNames, labels, formatters, ...props}: React.ComponentProps<typeof DayPicker> & {
    label: string;
}) {
    const {label, ...dayPickerProps} = props;
    return <DayPicker
        {...dayPickerProps}
        showOutsideDays
        showWeekNumber
        ISOWeek
        className={cn('calendar', className)}
        classNames={{...calendarClassNames, ...classNames}}
        labels={{
            ...labels,
            labelDayButton: date => dateKey(date),
            labelPrevious: () => `Previous month for ${label}`,
            labelNext: () => `Next month for ${label}`,
            labelWeekNumberHeader: () => 'CW',
        }}
        formatters={{
            ...formatters,
            formatWeekNumber: weekNumber => `CW${String(weekNumber).padStart(2, '0')}`,
            formatWeekNumberHeader: () => 'CW',
        }}
    />;
}
