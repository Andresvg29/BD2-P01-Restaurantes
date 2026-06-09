CREATE VIEW IF NOT EXISTS v_tendencias_mensuales AS
SELECT
    d.year,
    d.month,
    COUNT(DISTINCT fo.order_id) AS total_pedidos,
    ROUND(SUM(fo.total_amount), 2) AS ingresos_totales,
    COUNT(DISTINCT fo.user_key) AS usuarios_activos,
    SUM(CASE WHEN dos.status_name IN ('Cancelado', 'Cancelled') THEN 1 ELSE 0 END) AS pedidos_cancelados,
    ROUND(
        SUM(CASE WHEN dos.status_name IN ('Cancelado', 'Cancelled') THEN 1 ELSE 0 END) * 100.0
        / NULLIF(COUNT(DISTINCT fo.order_id), 0),
        2
    ) AS tasa_cancelacion_pct,
    ROUND(
        SUM(fo.total_amount)
        - LAG(SUM(fo.total_amount)) OVER (ORDER BY d.year, d.month),
        2
    ) AS diferencia_ingresos_mom,
    ROUND(
        (SUM(fo.total_amount) - LAG(SUM(fo.total_amount)) OVER (ORDER BY d.year, d.month))
        * 100.0 / NULLIF(LAG(SUM(fo.total_amount)) OVER (ORDER BY d.year, d.month), 0),
        2
    ) AS crecimiento_ingresos_pct
FROM fact_orders fo
JOIN dim_date d ON fo.date_key = d.date_key
JOIN dim_order_status dos ON fo.status_key = dos.status_key
GROUP BY d.year, d.month;
