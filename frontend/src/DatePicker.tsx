import {useEffect, useId, useRef, useState} from 'react';
import {isoWeek} from './calendarDate';
import {Calendar} from './components/ui/calendar';
import {Input} from './components/ui/input';
import {Label} from './components/ui/label';
import {Popover, PopoverAnchor, PopoverContent} from './components/ui/popover';

const pad = (n: number) => String(n).padStart(2, '0');
const dateKey = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
const parseDate = (value: string): Date | undefined => {
    if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return undefined;
    const date = new Date(`${value}T12:00:00`);
    return !Number.isNaN(date.getTime()) && dateKey(date) === value ? date : undefined;
};

type Props = {
    label: string;
    value: string;
    onChange: (value: string) => void;
    dateTime?: boolean;
    required?: boolean;
    autoFocus?: boolean;
    showWeekHint?: boolean;
    min?: string;
    max?: string;
};

export default function DatePicker({label, value: externalValue, onChange, dateTime = false, required, autoFocus, showWeekHint = true, min, max}: Props) {
    const [value, setValue] = useState(externalValue);
    useEffect(() => setValue(externalValue), [externalValue]);
    const changeValue = (next: string) => {setValue(next); onChange(next);};
    const [datePart = '', timePart = ''] = value.split('T');
    const id = useId();
    const input = useRef<HTMLInputElement>(null);
    const calendar = useRef<HTMLDivElement>(null);
    const focusCalendar = useRef(false);
    const [open, setOpen] = useState(false);
    const [month, setMonth] = useState(() => parseDate(value.slice(0, 10)) ?? new Date());
    const selected = parseDate(datePart);
    const validShape = dateTime ? /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2})?$/.test(value) : !!selected;
    const validTime = !dateTime || /^([01]\d|2[0-3]):[0-5]\d(:[0-5]\d)?$/.test(timePart);
    const comparable = (text: string) => dateTime && text.length === 16 ? `${text}:00` : text;
    const outOfRange = (min !== undefined && comparable(value) < comparable(min)) || (max !== undefined && comparable(value) > comparable(max));
    const invalid = !!value && (!selected || !validShape || !validTime || outOfRange);

    useEffect(() => {
        if (!open || !focusCalendar.current) return;
        focusCalendar.current = false;
        calendar.current?.querySelector<HTMLButtonElement>('[tabindex="0"]')?.focus();
    }, [open, month]);

    useEffect(() => {
        input.current?.setCustomValidity(invalid ? `Enter a valid ${dateTime ? 'date and time' : 'date'}${min ? ` at or after ${min}` : ''}${max ? ` at or before ${max}` : ''}.` : '');
    }, [invalid, dateTime, min, max]);

    const show = (focus = false) => {
        setMonth(selected ?? new Date());
        focusCalendar.current = focus;
        setOpen(true);
    };
    const choose = (day: Date | undefined) => {
        if (!day) return;
        const key = dateKey(day);
        changeValue(dateTime ? `${key}T${timePart}` : key);
        setOpen(false);
        input.current?.focus();
    };

    return <div className="date-picker" onKeyDown={event => {
        if (open && event.key === 'Escape') {
            event.preventDefault();
            event.stopPropagation();
            setOpen(false);
            input.current?.focus();
        }
    }}>
        <Label htmlFor={id}>{label}</Label>
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverAnchor asChild>
                <div className="date-picker-input">
                    <Input id={id} ref={input} type="text" required={required} autoFocus={autoFocus} value={datePart}
                        placeholder="YYYY-MM-DD" aria-invalid={invalid || undefined} aria-expanded={open}
                        aria-controls={`${id}-calendar`} aria-describedby={showWeekHint ? `${id}-hint` : undefined}
                        onClick={() => show()}
                        onChange={event => changeValue(dateTime ? `${event.target.value}T${timePart}` : event.target.value)}
                        onKeyDown={event => {if (event.key === 'ArrowDown') {event.preventDefault(); show(true);}}}/>
                </div>
            </PopoverAnchor>
            <PopoverContent ref={calendar} id={`${id}-calendar`} role="region" aria-label={`${label} calendar`}
                className="date-picker-calendar" align="start" onOpenAutoFocus={event => {
                    event.preventDefault();
                }} onInteractOutside={event => {
                    if (input.current && event.target instanceof Node && input.current.contains(event.target)) {
                        event.preventDefault();
                    }
                }} onEscapeKeyDown={event => {
                    event.preventDefault();
                    event.stopPropagation();
                    setOpen(false);
                    input.current?.focus();
                }}>
                <Calendar label={label} mode="single" selected={selected} month={month} onMonthChange={setMonth}
                    onSelect={choose} disabled={day =>
                        (min !== undefined && dateKey(day) < min.slice(0, 10)) ||
                        (max !== undefined && dateKey(day) > max.slice(0, 10))}/>
            </PopoverContent>
        </Popover>
        {dateTime && <Label className="date-picker-time">Time for {label}<Input type="time" step="1" required={required}
            value={validTime ? timePart : ''} aria-describedby={showWeekHint ? `${id}-hint` : undefined}
            onChange={event => changeValue(`${datePart}T${event.target.value}`)}/></Label>}
        {showWeekHint && <small id={`${id}-hint`} className="hint">{selected ? isoWeek(selected) : 'YYYY-MM-DD'}</small>}
        {invalid && <small className="error" role="alert">Enter a valid {dateTime ? 'date and time' : 'date'}{min ? ` at or after ${min}` : ''}{max ? ` at or before ${max}` : ''}.</small>}
    </div>;
}
