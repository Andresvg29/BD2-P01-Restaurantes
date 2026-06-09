CREATE VIEW IF NOT EXISTS v_reservas_por_ubicacion AS
SELECT
    r.ciudad,
    r.restaurant_key,
    r.nombre AS restaurant_nombre,
    d.year,
    d.month,
    COUNT(DISTINCT fr.reservation_id) AS total_reservas,
    SUM(fr.party_size) AS total_comensales,
    ROUND(AVG(fr.party_size), 2) AS promedio_personas,
    RANK() OVER (PARTITION BY r.ciudad ORDER BY COUNT(DISTINCT fr.reservation_id) DESC) AS ranking_ciudad
FROM fact_reservations fr
JOIN dim_date d ON fr.date_key = d.date_key
JOIN dim_restaurant r ON fr.restaurant_key = r.restaurant_key
GROUP BY r.ciudad, r.restaurant_key, r.nombre, d.year, d.month;
